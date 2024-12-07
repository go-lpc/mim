/*
 * Copyright 2020
 * Baptiste Joly <baptiste.joly@clermont.in2p3.fr>
 * Guillaume Blanchard <guillaume.blanchard@clermont.in2p3.fr>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston,
 * MA 02110-1301, USA.
 *
 */

/**
 * EDA board (Arrow SocKit) software library
 * I/O functions for communication with FPGA processes
 * via memory mapping
 */

#include "device.h"
#include "config.h"
#include "fpga.h"
#include "logger.h"

#include <arpa/inet.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>

#define PORT 8877
#define NB_READOUTS_PER_FILE 10000

char g_state = 1;

#include <signal.h>
void handle_sigint(int sig) {
  log_printf("Caught signal %d\n", sig);
  log_flush();
  g_state = 0;
}

int device_init_mmap(Device_t *ctx);
int device_init_fpga(Device_t *ctx);
int device_init_hrsc(Device_t *ctx);

int device_init_run(Device_t *ctx, uint32_t run);
void device_daq_write_dif(Device_t *ctx, int slot);
int device_daq_send_dif(Device_t *ctx, int slot);

void give_file_to_server(char *filename, int sock);

Device_t *new_device() {
  Device_t *ctx = (Device_t *)calloc(1, sizeof(Device_t));
  signal(SIGINT, handle_sigint);

  strcpy(ctx->cfg_mode, "csv");

  // fixed-location, temporary, line-buffered log file
  log_init();
  g_state = 1;

  return ctx;
}

void device_free(Device_t *ctx) {
  free(ctx->ip_addr);

  //  if (ctx->sock_cp != 0) {
  //    close(ctx->sock_cp);
  //  }
  //  if (ctx->sock_ctl != 0) {
  //    close(ctx->sock_ctl);
  //  }

  if (ctx->mem_fd != 0) {
    munmap_lw_h2f(ctx->mem_fd);
    munmap_h2f(ctx->mem_fd);
    close(ctx->mem_fd);
  }

  if (ctx->daq_file != 0) {
    fclose(ctx->daq_file);
  }

  for (int i = 0; i < NB_RFM; i++) {
    free(ctx->task[i].addr);
    if (ctx->task[i].sck != 0) {
      close(ctx->task[i].sck);
    }
  }

  free(ctx);
  ctx = NULL;
}

int device_boot_rfm(Device_t *ctx, uint8_t dif, int slot, uint32_t rshaper,
                    uint32_t trig) {
  ctx->rfm_on |= (1 << slot);
  ctx->task[slot].dif = dif;
  ctx->task[slot].rshaper = rshaper;

  if (ctx->rshaper != 0 && ctx->rshaper != rshaper) {
    log_printf("invalid rshaper value: device=%d, config=%d\n", ctx->rshaper,
               rshaper);
    log_flush();
    return -1;
  }
  ctx->rshaper = rshaper;
  ctx->trig_mode = trig;
  return 0;
}

int device_configure_dif(Device_t *ctx, uint8_t dif, const char *addr,
                         int port) {
  strcpy(ctx->cfg_mode, "db");
  for (int i = 0; i < NB_RFM; i++) {
    if (ctx->task[i].dif != dif) {
      continue;
    }
    free(ctx->task[i].addr);
    ctx->task[i].addr = strdup(addr);
    ctx->task[i].port = port;
    return 0;
  }
  return 1;
}

int device_configure(Device_t *ctx, uint32_t thresh, uint32_t rshaper,
                     uint32_t rfm, const char *ip, int run) {
  log_printf("device configuration from %s...\n", ctx->cfg_mode);
  log_flush();

  ctx->thresh_delta = thresh;
  ctx->rshaper = rshaper;
  ctx->rfm_on = rfm;
  ctx->ip_addr = strdup(ip);
  ctx->run_cnt = run;

  // copy base settings files from clrtodaq0 (using ssh keys)
  char command[128];
  sprintf(
      command,
      "scp -P 1122 -r mim@193.48.81.203:/mim/soft/eda/config_base /dev/shm/");

  int err = 0;
  err = system(command);
  if (err != 0) {
    log_printf("could not copy base settings from clrtodaq\n");
    return err;
  }

  // load files to tables
  // single-HR configuration file
  FILE *conf_base_file = fopen("/dev/shm/config_base/conf_base.csv", "r");
  if (!conf_base_file)
    return -1;
  if (HRSC_read_conf_singl(conf_base_file, 0) < 0) {
    fclose(conf_base_file);
    return -1;
  }
  fclose(conf_base_file);

  // floor thresholds
  FILE *dac_floor_file = fopen("/dev/shm/config_base/dac_floor_4rfm.csv", "r");
  if (!dac_floor_file)
    return -1;
  if (read_th_offset(dac_floor_file, ctx->dac_floor_table) < 0) {
    fclose(dac_floor_file);
    return -1;
  }
  fclose(dac_floor_file);

  // preamplifier gains
  FILE *pa_gain_file = fopen("/dev/shm/config_base/pa_gain_4rfm.csv", "r");
  if (!pa_gain_file)
    return -1;
  if (read_pa_gain(pa_gain_file, ctx->pa_gain_table) < 0) {
    fclose(pa_gain_file);
    return -1;
  }
  fclose(pa_gain_file);

  // masks
  FILE *mask_file = fopen("/dev/shm/config_base/mask_4rfm.csv", "r");
  if (!mask_file)
    return -1;
  if (read_mask(mask_file, ctx->mask_table) < 0) {
    fclose(mask_file);
    return -1;
  }
  fclose(mask_file);

  return 0;
}

int device_initialize(Device_t *ctx) {
  int err = 0;

  // FPGA-HPS memory mapping-------------------------------------------
  err = device_init_mmap(ctx);
  if (err != 0) {
    log_printf("could not initialize mmap\n");
    log_flush();
    return -1;
  }

  // Init FPGA---------------------------------------------------------
  err = device_init_fpga(ctx);
  if (err != 0) {
    log_printf("could not initialize fpga\n");
    log_flush();
    return -1;
  }

  // HR configuration--------------------------------------------------
  err = device_init_hrsc(ctx);
  if (err != 0) {
    log_printf("could not initialize hrsc\n");
    log_flush();
    return -1;
  }

  // initialize DIM<->EDA data sockets.
  err = device_init_scks(ctx);
  if (err != 0) {
    log_printf("could not initialize scks\n");
    log_flush();
    return -1;
  }

  return 0;
}

int device_init_scks(Device_t *ctx) {
  for (int i = 0; i < NB_RFM; i++) {
    if (ctx->task[i].dif == 0) {
      continue;
    }
    log_printf(
        "initialize DIM<->RFM data socket rfm=%d, slot=%d, addr=%s:%d...\n",
        ctx->task[i].dif, i, ctx->task[i].addr, ctx->task[i].port);
    log_flush();

    struct sockaddr_in addr;
    if ((ctx->task[i].sck = socket(AF_INET, SOCK_STREAM, 0)) < 0) {
      log_printf("could not create socket for rfm=%d, slot=%d\n",
                 ctx->task[i].dif, i);
      log_flush();
      return -1;
    }

    addr.sin_family = AF_INET;
    addr.sin_port = htons(ctx->task[i].port);

    // Convert IPv4 and IPv6 addresses from text to binary form
    if (inet_pton(AF_INET, ctx->task[i].addr, &addr.sin_addr) <= 0) {
      log_printf("could not convert socket-addr for rfm=%d, slot=%d\n",
                 ctx->task[i].dif, i);
      log_flush();
      return -1;
    }

    if (connect(ctx->task[i].sck, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
      log_printf("could not connect socket for rfm=%d, slot=%d\n",
                 ctx->task[i].dif, i);
      log_flush();
      return -1;
    }
  }

  return 0;
}

int device_init_mmap(Device_t *ctx) {
  ctx->mem_fd = 0;
  if ((ctx->mem_fd = open("/dev/mem", (O_RDWR | O_SYNC))) == -1) {
    log_printf("ERROR: could not open \"/dev/mem\"...\n");
    log_flush();
    ctx->mem_fd = 0;
    return -1;
  }
  // lightweight HPS to FPGA bus
  if (!mmap_lw_h2f(ctx->mem_fd)) {
    log_printf("could not mmap lw HPS to FPGA bus\n");
    log_flush();
    close(ctx->mem_fd);
    ctx->mem_fd = 0;
    return -1;
  }
  // HPS to FPGA bus
  if (!mmap_h2f(ctx->mem_fd)) {
    log_printf("could not mmap HPS to FPGA bus\n");
    log_flush();
    munmap_lw_h2f(ctx->mem_fd);
    close(ctx->mem_fd);
    ctx->mem_fd = 0;
    return -1;
  }

  return 0;
}

int device_init_fpga(Device_t *ctx) {
  // reset fpga and set clock
  SYNC_reset_fpga();
  usleep(2);
  // make sure the pll is locked
  int cnt_poll = 0;
  while ((!SYNC_pll_lck()) && (cnt_poll < 100)) {
    usleep(10000);
    cnt_poll++;
  }
  if (cnt_poll >= 100) {
    log_printf("the PLL is not locked\n");
    log_flush();
    return -1;
  }
  log_printf("the PLL is locked\n");
  log_printf("pll lock=%d\n", SYNC_pll_lck());
  log_flush();

  // activate RFMs
  int rfm_index;
  for (rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
    if (((ctx->rfm_on >> rfm_index) & 1) == 1) {
      RFM_on(rfm_index);
      RFM_enable(rfm_index);
    }
  }
  sleep(1);
  log_printf("control pio=%lx\n", PIO_ctrl_get());
  log_flush();

  log_printf("trigger mode: %d\n", ctx->trig_mode);
  log_flush();
  if (ctx->trig_mode == 0) {
    SYNC_select_command_dcc();
    SYNC_enable_dcc_busy();
    SYNC_enable_dcc_ramfull();
  }
  if (ctx->trig_mode == 1) {
    SYNC_select_command_soft();
  }

  return 0;
}

int device_init_hrsc_from_csv(Device_t *ctx);
int device_init_hrsc_from_db(Device_t *ctx);

int device_init_hrsc(Device_t *ctx) {
  log_printf("Hardroc configuration from %s...\n", ctx->cfg_mode);
  log_flush();

  // disable trig_out output pin (RFM v1 coupling problem)
  HRSC_set_bit(0, 854, 0);

  HRSC_set_shaper_resis(0, ctx->rshaper);
  HRSC_set_shaper_capa(0, 3);

  // set chip ids
  for (alt_u32 hr_addr = 0; hr_addr < 8; hr_addr++) {
    HRSC_set_chip_id(hr_addr, hr_addr + 1);
  }

  if (strcmp(ctx->cfg_mode, "db") == 0) {
    return device_init_hrsc_from_db(ctx);
  }

  if (strcmp(ctx->cfg_mode, "csv") == 0) {
    return device_init_hrsc_from_csv(ctx);
  }

  log_printf("invalid configuration mode %q\n", ctx->cfg_mode);
  log_flush();
  return -1;
}

int device_init_hrsc_from_db(Device_t *ctx) {
  alt_u32 hr_addr, chan;
  // for each active RFM, tune the configuration and send it
  for (int rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
    if (((ctx->rfm_on >> rfm_index) & 1) == 1) {
      // set mask
      alt_u32 mask;
      for (hr_addr = 0; hr_addr < 8; hr_addr++) {
        for (chan = 0; chan < 64; chan++) {
          mask = ctx->mask_table[64 * (NB_HR * rfm_index + hr_addr) + chan];
          HRSC_set_mask(hr_addr, chan, mask);
        }
      }
      // set DAC thresholds
      alt_u32 th0, th1, th2;
      for (hr_addr = 0; hr_addr < 8; hr_addr++) {
        th0 = ctx->dac_floor_table[3 * (NB_HR * rfm_index + hr_addr) + 0];
        th1 = ctx->dac_floor_table[3 * (NB_HR * rfm_index + hr_addr) + 1];
        th2 = ctx->dac_floor_table[3 * (NB_HR * rfm_index + hr_addr) + 2];
        HRSC_set_DAC0(hr_addr, th0);
        HRSC_set_DAC1(hr_addr, th1);
        HRSC_set_DAC2(hr_addr, th2);
      }
      // set preamplifier gain
      alt_u32 pa_gain;
      for (hr_addr = 0; hr_addr < 8; hr_addr++) {
        for (chan = 0; chan < 64; chan++) {
          pa_gain =
              ctx->pa_gain_table[64 * (NB_HR * rfm_index + hr_addr) + chan];
          HRSC_set_preamp(hr_addr, chan, pa_gain);
        }
      }
      // send to HRs
      if (HRSC_set_config(rfm_index) < 0) {
        PRINT_config(stderr, rfm_index);
        return -1;
      }
      log_printf("Hardroc configuration done (rfm=%d, slot=%d)\n",
                 ctx->task[rfm_index].dif, rfm_index);
      log_flush();
      if (HRSC_reset_read_registers(rfm_index) < 0) {
        PRINT_config(stderr, rfm_index);
        return -1;
      }
    }
  }

  log_printf("read register reset done\n");
  log_flush();
  sleep(1); // let DACs stabilize
  log_printf("Hardroc configuration from %s... [done]\n", ctx->cfg_mode);
  log_flush();
  return 0;
}

int device_init_hrsc_from_csv(Device_t *ctx) {
  int err = 0;

  HRSC_copy_conf(0, 1);
  HRSC_copy_conf(0, 2);
  HRSC_copy_conf(0, 3);
  HRSC_copy_conf(0, 4);
  HRSC_copy_conf(0, 5);
  HRSC_copy_conf(0, 6);
  HRSC_copy_conf(0, 7);

  // prepare config file (for history)
  char sc_filename[128];
  sprintf(sc_filename, "/home/root/run/hr_sc_%03d.csv", ctx->run_cnt);
  FILE *sc_file = fopen(sc_filename, "w");
  if (!sc_file) {
    log_printf("could not open file %s\n", sc_filename);
    log_flush();
    return -1;
  }

  alt_u32 hr_addr, chan;

  // for each active RFM, tune the configuration and send it
  for (int rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
    if (((ctx->rfm_on >> rfm_index) & 1) == 1) {
      // set mask
      alt_u32 mask;
      for (hr_addr = 0; hr_addr < 8; hr_addr++) {
        for (chan = 0; chan < 64; chan++) {
          mask = ctx->mask_table[64 * (NB_HR * rfm_index + hr_addr) + chan];
          log_printf("%u      %u      %u\n", (uint32_t)hr_addr, (uint32_t)chan,
                     (uint32_t)mask);
          log_flush();
          HRSC_set_mask(hr_addr, chan, mask);
        }
      }
      // set DAC thresholds
      log_printf("HR      thresh0     thresh1     thresh2\n");
      log_flush();
      alt_u32 th0, th1, th2;
      for (hr_addr = 0; hr_addr < 8; hr_addr++) {
        th0 = ctx->dac_floor_table[3 * (NB_HR * rfm_index + hr_addr)] +
              ctx->thresh_delta;
        th1 = ctx->dac_floor_table[3 * (NB_HR * rfm_index + hr_addr) + 1] +
              ctx->thresh_delta;
        th2 = ctx->dac_floor_table[3 * (NB_HR * rfm_index + hr_addr) + 2] +
              ctx->thresh_delta;
        log_printf("%u      %u      %u      %u\n", (uint32_t)hr_addr,
                   (uint32_t)th0, (uint32_t)th1, (uint32_t)th2);
        log_flush();
        HRSC_set_DAC0(hr_addr, th0);
        HRSC_set_DAC1(hr_addr, th1);
        HRSC_set_DAC2(hr_addr, th2);
      }
      // set preamplifier gain
      log_printf("HR      chan        pa_gain\n");
      log_flush();
      alt_u32 pa_gain;
      for (hr_addr = 0; hr_addr < 8; hr_addr++) {
        for (chan = 0; chan < 64; chan++) {
          pa_gain =
              ctx->pa_gain_table[64 * (NB_HR * rfm_index + hr_addr) + chan];
          log_printf("%u      %u      %u\n", (uint32_t)hr_addr, (uint32_t)chan,
                     (uint32_t)pa_gain);
          log_flush();
          HRSC_set_preamp(hr_addr, chan, pa_gain);
        }
      }
      // send to HRs
      if (HRSC_set_config(rfm_index) < 0) {
        PRINT_config(stderr, rfm_index);
        return -1;
      }
      log_printf("Hardroc configuration done\n");
      log_flush();
      if (HRSC_reset_read_registers(rfm_index) < 0) {
        PRINT_config(stderr, rfm_index);
        return -1;
      }
      fprintf(sc_file, "#RFM_INDEX= %d ------------------------\n", rfm_index);
      HRSC_write_conf_mult(sc_file);
    }
  }
  fclose(sc_file);
  char command[256];
  sprintf(command,
          "scp -P 1122 %s mim@193.48.81.203:/mim/soft/eda/config_history/",
          sc_filename);
  err = system(command);
  if (err != 0) {
    log_printf("could not send config to history store: err=%d\n", err);
    log_flush();
    return err;
  }

  log_printf("read register reset done\n");
  log_flush();
  sleep(1); // let DACs stabilize

  log_printf("Hardroc configuration from %s... [done]\n", ctx->cfg_mode);
  log_flush();
  return 0;
}

int device_start_run_noise(Device_t *ctx, uint32_t run);
int device_start_run_dcc(Device_t *ctx, uint32_t run);

int device_start(Device_t *ctx, uint32_t run) {
  int err = 0;

  // init run----------------------------------------------------------
  err = device_init_run(ctx, run);
  if (err != 0) {
    return err;
  }

  if (ctx->trig_mode == 0) {
    return device_start_run_dcc(ctx, run);
  }
  if (ctx->trig_mode == 1) {
    return device_start_run_noise(ctx, run);
  }
  log_printf("unknown trig-mode: %d\n", ctx->trig_mode);
  log_flush();
  return -1;
}

int device_start_run_dcc(Device_t *ctx, uint32_t run) {
  log_printf("start-run(%d) mode=dcc...\n", run);
  log_flush();
  // wait for reset BCID
  log_printf("waiting for reset_BCID command\n");
  log_flush();
  //  send(ctx->sock_ctl, "eda-ready", 9, 0);

  int dcc_cmd = 0xE;
  while (dcc_cmd != CMD_RESET_BCID) {
    while (SYNC_dcc_cmd_mem() == dcc_cmd) {
      if (g_state == 0)
        break;
    }
    dcc_cmd = SYNC_dcc_cmd_mem();
    // log_printf("sDCC command = %d\n",dcc_cmd);
    if (g_state == 0)
      break;
  }
  if (g_state == 0) {
    return -1;
  }
  log_printf("SYNC_state()=%d\n", SYNC_state());
  log_printf("reset_BCID done\n");
  log_flush();

  CNT_reset();
  CNT_start();
  for (int rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
    if (((ctx->rfm_on >> rfm_index) & 1) == 1) {
      DAQ_fifo_init(rfm_index);
    }
  }

  ctx->cycle_id = 0;
  SYNC_fifo_arming();

  //  for (int i = 0; i < NB_RFM; i++) {
  //    if (((ctx->rfm_on >> i) & 1) == 0) {
  //      continue;
  //    }
  //    err = pthread_create(&ctx->task[i].id, NULL, device_loop, (void
  //    *)ctx); if (err != 0) {
  //      log_printf("could not create worker for RFM=%d: err=%d\n", i, err);
  //      log_flush();
  //      return err;
  //    }
  //  }

  //  err = pthread_create(&ctx->task[0].id, NULL, device_loop, (void *)ctx);
  //  if (err != 0) {
  //    log_printf("could not create worker: err=%d\n", err);
  //    log_flush();
  //    return err;
  //  }

  return 0;
}

int device_start_run_noise(Device_t *ctx, uint32_t run) {
  log_printf("start-run(%d) mode=noise...\n", run);
  log_flush();
  for (int i = 0; i < NB_RFM; i++) {
    if (((ctx->rfm_on >> i) & 1) == 1) {
      DAQ_fifo_init(i);
    }
  }

  CNT_reset();
  SYNC_reset_bcid();
  SYNC_start_acq();
  log_printf("SYNC_state()=%d\n", SYNC_state());
  log_printf("reset_BCID done\n");
  log_flush();

  ctx->cycle_id = 0;
  SYNC_fifo_arming();

  return 0;
}

int device_init_run(Device_t *ctx, uint32_t run) {
  // save run-dependant settings
  ctx->run_cnt = run;
  log_printf("thresh_delta=%lu, Rshaper=%lu, rfm_on[3:0]=%lu\n",
             ctx->thresh_delta, ctx->rshaper, ctx->rfm_on);
  log_flush();
  //  char settings_filename[128];
  //  sprintf(settings_filename, "/home/root/run/settings_%03d.csv",
  //  ctx->run_cnt); FILE *settings_file = fopen(settings_filename, "w"); if
  //  (!settings_file) {
  //    log_printf("could not open file %s\n", settings_filename);
  //    log_flush();
  //    return -1;
  //  }
  //  fprintf(
  //      settings_file,
  //      "thresh_delta=%lu; Rshaper=%lu; rfm_on[3:0]=%lu; ip_addr=%s;
  //      run_id=%d", ctx->thresh_delta, ctx->rshaper, ctx->rfm_on,
  //      ctx->ip_addr, ctx->run_cnt);
  //  fclose(settings_file);
  //  give_file_to_server(settings_filename, ctx->sock_cp);

  log_printf("-----------------RUN NB %d-----------------\n", ctx->run_cnt);
  log_flush();

  // FIXME(sbinet): replace with a pair of open_memstream(3).
  // prepare run file
  sprintf(ctx->daq_filename, "/dev/shm/eda_%03d.000.raw",
          ctx->run_cnt); // use tmpfs for daq (to reduce writings on µSD flash
                         // mem)
  ctx->daq_file = fopen(ctx->daq_filename, "w");
  if (!ctx->daq_file) {
    log_printf("unable to open file %s\n", ctx->daq_filename);
    log_flush();
    return -1;
  }
  // init run counters
  ctx->cycle_id = 0;

  SYNC_reset_hr();

  return 0;
}

void device_loop_noise(Device_t *ctx);
void device_loop_dcc(Device_t *ctx);

void device_loop(Device_t *ctx) {
  if (ctx->trig_mode == 0) {
    device_loop_dcc(ctx);
    return;
  }
  if (ctx->trig_mode == 1) {
    device_loop_noise(ctx);
    return;
  }
}

void device_loop_dcc(Device_t *ctx) {
  int err = 0;
  // int file_cnt = 0;
  //----------- run loop -----------------
  while (g_state == 1) {
    DAQ_reset_buffer();
    // wait until new readout is started
    log_printf("trigger %07lu\n\tacq\n", ctx->cycle_id);
    log_flush();
    while ((SYNC_fpga_ro() == 0) && (g_state == 1)) {
      ;
    }
    if (g_state == 0) {
      break;
    }

    log_printf("\treadout\n");
    log_flush();
    // wait until readout is done
    while ((SYNC_fifo_ready() == 0) && (g_state == 1)) {
      ;
    }
    if (g_state == 0) {
      break;
    }

    // read hardroc data
    log_printf("\tbuffering\n");
    log_flush();
    for (int rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
      if (((ctx->rfm_on >> rfm_index) & 1) == 0) {
        continue;
      }
      log_printf("\t\trfm %d\n", rfm_index);
      log_flush();
      device_daq_write_dif(ctx, rfm_index);
      if (g_state == 0) {
        break;
      }
    }
    SYNC_fifo_ack();

    // write data file
    log_printf("\tfwrite\n");
    log_flush();
    DAQ_write_buffer(ctx->daq_file);
    log_printf("\tdone\n");
    log_flush();
    for (int i = 0; i < NB_RFM; i++) {
      if (((ctx->rfm_on >> i) & 1) == 0) {
        continue;
      }
      err = device_daq_send_dif(ctx, i);
      if (err != 0) {
        log_printf("\tcould not send dif data RFM=%d, slot=%d: err=%d\n",
                   ctx->task[i].dif, i, err);
        ctx->task[i].rc = err;
      }
    }

    ctx->cycle_id++;
    //    if ((ctx->cycle_id % NB_READOUTS_PER_FILE) == 0) {
    //      fclose(ctx->daq_file);
    //      give_file_to_server(ctx->daq_filename, ctx->sock_cp);
    //      // prepare new file
    //      memset(ctx->daq_filename, 0, 128);
    //      file_cnt++;
    //      sprintf(ctx->daq_filename, "/dev/shm/eda_%03d.%03d.raw",
    //      ctx->run_cnt,
    //              file_cnt);
    //      ctx->daq_file = fopen(ctx->daq_filename, "w");
    //    }
  }

  return;
}

void device_loop_noise(Device_t *ctx) {
  int err = 0;
  while (g_state == 1) {
    DAQ_reset_buffer();
    log_printf("trigger %07lu\n\tacq\n", ctx->cycle_id);
    log_flush();
    // wait for ramfull
    while ((SYNC_ramfull() == 0) && (g_state == 1)) {
      ;
    }
    if (g_state == 0)
      break;
    log_printf("\tramfull\n");
    log_flush();
    SYNC_ramfull_ext();
    // wait until data is ready
    while ((SYNC_fifo_ready() == 0) && (g_state == 1)) {
      ;
    }
    if (g_state == 0) {
      break;
    }
    // read hardroc data
    log_printf("\tbuffering\n");
    log_flush();
    for (int rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
      if (((ctx->rfm_on >> rfm_index) & 1) == 0) {
        continue;
      }
      log_printf("\t\trfm %d\n", rfm_index);
      log_flush();
      device_daq_write_dif(ctx, rfm_index);
      if (g_state == 0) {
        break;
      }
    }
    SYNC_fifo_ack();
    log_printf("\tdone\n");
    log_flush();
    for (int i = 0; i < NB_RFM; i++) {
      if (((ctx->rfm_on >> i) & 1) == 0) {
        continue;
      }
      err = device_daq_send_dif(ctx, i);
      if (err != 0) {
        log_printf("\tcould not send dif data RFM=%d, slot=%d: err=%d\n",
                   ctx->task[i].dif, i, err);
        ctx->task[i].rc = err;
      }
    }

    SYNC_start_acq();
    ctx->cycle_id++;
  }
}

int device_daq_send_dif(Device_t *ctx, int slot) {
  int err = 0;
  int ret = 0;
  int sck = ctx->task[slot].sck;
  uint8_t buf[8] = {'H', 'D', 'R', 0, 0, 0, 0, 0};
  uint8_t *data = ctx->task[slot].beg;
  uint32_t size = ctx->task[slot].end - ctx->task[slot].beg;

  buf[4 + 0] = (uint8_t)(size);
  buf[4 + 1] = (uint8_t)(size >> 8);
  buf[4 + 2] = (uint8_t)(size >> 16);
  buf[4 + 3] = (uint8_t)(size >> 24);

  err = send(sck, buf, 8, 0);
  if (err == -1) {
    log_printf("could not send DIF header (rfm=%d, slot=%d): err=%d\n",
               ctx->task[slot].dif, slot, err);
    log_flush();
    return err;
  }

  ret = recv(sck, buf, 4, 0);
  if (ret != 4 || (strcmp((const char *)buf, "ACK") != 0)) {
    log_printf("could not recv HDR-ACK (rfm=%d, slot=%d): sz=%d buf=%s\n",
               ctx->task[slot].dif, slot, ret, (const char *)buf);
    log_flush();
    return 1;
  }

  if (size == 0) {
    return 0;
  }

  err = send(sck, data, size, 0);
  if (err == -1) {
    log_printf("could not send DIF data (rfm=%d, slot=%d): err=%d\n",
               ctx->task[slot].dif, slot, err);
    log_flush();
  }

  ret = recv(sck, buf, 4, 0);
  if (ret != 4 || (strcmp((const char *)buf, "ACK") != 0)) {
    log_printf(
        "could not recv DIF data (rfm=%d, slot=%d) ACK: size=%d, buf=%s\n",
        ctx->task[slot].dif, slot, ret, (const char *)buf);
    log_flush();
    return 1;
  }

  return 0;
}

int device_stop(Device_t *ctx) {
  if (ctx->trig_mode == 0) {
    CNT_stop();
  }
  if (ctx->trig_mode == 1) {
    SYNC_stop_acq();
    CNT_stop();
  }
  CNT_reset();
  SYNC_reset_fpga();
  SYNC_reset_hr();

  // close current daq file
  fclose(ctx->daq_file);
  ctx->daq_file = NULL;

  //  give_file_to_server(ctx->daq_filename, ctx->sock_cp);
  return 0;
}

void device_stop_loop(Device_t *ctx) {
  g_state = 0; // FIXME(sbinet): r/w race.
}

void give_file_to_server(char *filename, int sock) {
  if (sock == 0) {
    return;
  }
  log_printf("send copy request to eda-srv\n");
  log_flush();
  alt_u32 length;
  alt_u8 length_litend[4] = {0};
  char sock_read_buf[4] = {0};
  int valread;
  // send lenght of filename (uint32 little endian) to server
  length = strlen(filename);
  length_litend[0] = length; // length < 128 < 256
  send(sock, length_litend, 4, 0);
  // send filename to server
  send(sock, filename, length, 0);
  // wait server ack
  valread = read(sock, sock_read_buf, 3);
  if ((valread != 3) || (strcmp(sock_read_buf, "ACK") != 0)) {
    log_printf("instead of ACK, received :%s\n", sock_read_buf);
    log_flush();
  }
  return;
}
