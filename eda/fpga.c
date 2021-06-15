// Copyright 2021 The tomuvol-daq Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <arpa/inet.h>
#include <fcntl.h>
#include <linux/i2c-dev.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <unistd.h>

#include "device.h"
#include "fpga.h"

// static variables, with file-scope
// are persistent between function calls and
// can be modified by all the functions below

static void *virtual_lw_h2f;
static void *virtual_h2f;
static int i2c_file;

static alt_u32 *p_pio_state;
static alt_u32 *p_pio_irq_bus;
static alt_u32 *p_pio_irq_mask;
// static alt_u32 *p_pio_irq_context;
static alt_u32 *p_pio_ctrl;
static alt_u32 *p_pio_pulser;

static alt_u32 *p_ram_sc[4];
static alt_u32 *p_pio_sc_check[4];

static alt_u32 *p_fifo_daq[4];
static alt_u32 *p_fifo_daq_csr[4];

static alt_u32 *p_pio_cnt_hit0[4];
static alt_u32 *p_pio_cnt_hit1[4];
static alt_u32 *p_pio_cnt_trig;
static alt_u32 *p_pio_cnt48_msb;
static alt_u32 *p_pio_cnt48_lsb;
static alt_u32 *p_pio_cnt24;

static alt_u8 this_eda_id = 1; // 0 to 7
static alt_u32 daq_cycle_id[4] = {0, 0, 0, 0};

static uint8_t hr_config_buffer[NB_HR * NB_BYTES_CFG_HR + 4] = {0};
static uint8_t *hr_config_data = &(hr_config_buffer[4]);

static uint8_t hr_data_buffer[NB_RFM * (26 + NB_HR * (2 + 128 * 20))] = {0};
static uint8_t *hr_data_cursor = hr_data_buffer;

#define MASK_1_BIT 0x1

/************** memory-mapping *****************/

int mmap_lw_h2f(int fd) {
  virtual_lw_h2f = mmap(NULL, LW_H2F_SPAN, (PROT_READ | PROT_WRITE), MAP_SHARED,
                        fd, LW_H2F_BASE);
  if (virtual_lw_h2f == MAP_FAILED) {
    fprintf(stderr,
            "ERROR: mmap() failed for lightweight HPS to FPGA bridge...\n");
    return (-1);
  }
  p_pio_state = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_STATE_IN);
  p_pio_irq_bus =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_INTERRUPTS_BUS);
  p_pio_irq_mask =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_INTERRUPTS_MASK);
  p_pio_ctrl = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CTRL_OUT);
  p_pio_pulser = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_PULSER);

  p_ram_sc[0] = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_RAM_SC_RFM0);
  p_ram_sc[1] = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_RAM_SC_RFM1);
  p_ram_sc[2] = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_RAM_SC_RFM2);
  p_ram_sc[3] = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_RAM_SC_RFM3);

  p_pio_sc_check[0] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_SC_CHECK_RFM0);
  p_pio_sc_check[1] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_SC_CHECK_RFM1);
  p_pio_sc_check[2] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_SC_CHECK_RFM2);
  p_pio_sc_check[3] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_SC_CHECK_RFM3);

  p_pio_cnt_hit0[0] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT0_RFM0);
  p_pio_cnt_hit0[1] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT0_RFM1);
  p_pio_cnt_hit0[2] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT0_RFM2);
  p_pio_cnt_hit0[3] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT0_RFM3);

  p_pio_cnt_hit1[0] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT1_RFM0);
  p_pio_cnt_hit1[1] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT1_RFM1);
  p_pio_cnt_hit1[2] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT1_RFM2);
  p_pio_cnt_hit1[3] =
      (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_HIT1_RFM3);

  p_pio_cnt_trig = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT_TRIG);
  p_pio_cnt48_msb = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT48_MSB);
  p_pio_cnt48_lsb = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT48_LSB);
  p_pio_cnt24 = (alt_u32 *)((alt_u32)virtual_lw_h2f + LW_H2F_PIO_CNT24);
  return (1);
}

int mmap_h2f(int fd) {
  virtual_h2f =
      mmap(NULL, H2F_SPAN, (PROT_READ | PROT_WRITE), MAP_SHARED, fd, H2F_BASE);
  if (virtual_h2f == MAP_FAILED) {
    fprintf(stderr, "ERROR: mmap() failed for HPS to FPGA bridge...\n");
    return (-1);
  }
  p_fifo_daq[0] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_RFM0);
  p_fifo_daq[1] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_RFM1);
  p_fifo_daq[2] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_RFM2);
  p_fifo_daq[3] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_RFM3);

  p_fifo_daq_csr[0] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_CSR_RFM0);
  p_fifo_daq_csr[1] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_CSR_RFM1);
  p_fifo_daq_csr[2] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_CSR_RFM2);
  p_fifo_daq_csr[3] = (alt_u32 *)((alt_u32)virtual_h2f + H2F_FIFO_DAQ_CSR_RFM3);

  return (1);
}

int munmap_lw_h2f(int fd) {
  if (munmap(virtual_lw_h2f, LW_H2F_SPAN) != 0) {
    fprintf(stderr, "ERROR: munmap() failed for lw_h2f...\n");
    return (-1);
  }
  return (1);
}

int munmap_h2f(int fd) {
  if (munmap(virtual_h2f, H2F_SPAN) != 0) {
    fprintf(stderr, "ERROR: munmap() failed for h2f...\n");
    return (-1);
  }
  return (1);
}

// binary write functions -- big endian, unsigned, 8/16/32 bits

ssize_t fwrite_u8(alt_u8 byte, FILE *output) {
  return fwrite(&byte, 1, 1, output);
}

ssize_t fwrite_u16(alt_u16 word16, FILE *output) {
  alt_u16 bigendian16 = htons(word16);
  return fwrite(&bigendian16, 2, 1, output);
}

ssize_t fwrite_u24(alt_u32 word32, FILE *output) {
  alt_u8 word8 = (word32 >> 16) & 0xFF;
  alt_u16 word16 = word32 & 0xFFFF;
  ssize_t ret = fwrite_u8(word8, output);
  ret += fwrite_u16(word16, output);
  return ret;
}

ssize_t fwrite_u32(alt_u32 word32, FILE *output) {
  alt_u32 bigendian32 = htonl(word32);
  return fwrite(&bigendian32, 4, 1, output);
}

/************** pio general *****************/

void PIO_set_defaults() { *p_pio_ctrl = 0; }

alt_u32 PIO_state_get() { return (*p_pio_state); }

alt_u32 PIO_ctrl_get() { return (*p_pio_ctrl); }

// static irqreturn_t PIO_handle_interrupts(int irq, void *dev_id)
//{
// if (irq != PIO_IRQ)
// return IRQ_NONE;
///* Store the value of *p_pio_irq_bus in *p_pio_irq_context. */
//*p_pio_irq_context = *p_pio_irq_bus;
///* mask the bit that caused the interrupt, unmask the others */
//*p_pio_irq_mask = ~(*p_pio_irq_bus) & 0xf;
// fprintf(stderr,"----interrupt, state=");
// switch(*irq_bus){
// case IRQ_RST_BCID:
// fprintf(stderr,"reset_bcid\n");
// break;
// case IRQ_ACQ:
// fprintf(stderr,"acq\n");
// break;
// case IRQ_RO:
// fprintf(stderr,"readout\n");
// break;
// case IRQ_FIFO_READY:
// fprintf(stderr,"fifo_ready\n");
// break;
// default:
// fprintf(stderr,"illegal state\n");
// break;
//}
// return IRQ_HANDLED;
//}

// PIO_interrupts_init(){
//*p_pio_irq_mask = 0xf; //enable all 4 bits
// request_irq(PIO_IRQ, PIO_handle_interrupts,
// 0, "state_machine_interrupts", NULL);
//}

/************** i2c general *****************/

int I2C_init() {
  if ((i2c_file = open("/dev/i2c-0", O_RDWR)) < 0) {
    /* ERROR HANDLING: you can check errno to see what went wrong */
    perror("Failed to open the i2c bus of gsensor");
    return (-1);
  }
  return (1);
}

void I2C_close() { close(i2c_file); }

/********************** RFM on-off, enable *********************/

void RFM_on(int rfm) {
  switch (rfm) {
  case 0:
    *p_pio_ctrl |= O_ON_OFF_RFM0;
    return;
  case 1:
    *p_pio_ctrl |= O_ON_OFF_RFM1;
    return;
  case 2:
    *p_pio_ctrl |= O_ON_OFF_RFM2;
    return;
  case 3:
    *p_pio_ctrl |= O_ON_OFF_RFM3;
    return;
  default:
    fprintf(stderr, "rfm_on: bad argument %d\n", rfm);
    return;
  }
};

void RFM_off(int rfm) {
  switch (rfm) {
  case 0:
    *p_pio_ctrl &= ~O_ON_OFF_RFM0;
    return;
  case 1:
    *p_pio_ctrl &= ~O_ON_OFF_RFM1;
    return;
  case 2:
    *p_pio_ctrl &= ~O_ON_OFF_RFM2;
    return;
  case 3:
    *p_pio_ctrl &= ~O_ON_OFF_RFM3;
    return;
  default:
    fprintf(stderr, "rfm_off: bad argument %d\n", rfm);
    return;
  }
};

void RFM_enable(int rfm) {
  switch (rfm) {
  case 0:
    *p_pio_ctrl |= O_ENA_RFM0;
    return;
  case 1:
    *p_pio_ctrl |= O_ENA_RFM1;
    return;
  case 2:
    *p_pio_ctrl |= O_ENA_RFM2;
    return;
  case 3:
    *p_pio_ctrl |= O_ENA_RFM3;
    return;
  default:
    fprintf(stderr, "rfm_enable: bad argument %d\n", rfm);
    return;
  }
};

void RFM_disable(int rfm) {
  switch (rfm) {
  case 0:
    *p_pio_ctrl &= ~O_ENA_RFM0;
    return;
  case 1:
    *p_pio_ctrl &= ~O_ENA_RFM1;
    return;
  case 2:
    *p_pio_ctrl &= ~O_ENA_RFM2;
    return;
  case 3:
    *p_pio_ctrl &= ~O_ENA_RFM3;
    return;
  default:
    fprintf(stderr, "rfm_disable: bad argument %d\n", rfm);
    return;
  }
};

/********************** monitoring *********************/

int MONIT_pwr_alert(int rfm) {
  switch (rfm) {
  case 0:
    return ((*p_pio_state & O_ALERT_0) == O_ALERT_0);
  case 1:
    return ((*p_pio_state & O_ALERT_1) == O_ALERT_1);
  case 2:
    return ((*p_pio_state & O_ALERT_2) == O_ALERT_2);
  case 3:
    return ((*p_pio_state & O_ALERT_3) == O_ALERT_3);
  default:
    fprintf(stderr, "pwr_alert: bad argument %d\n", rfm);
    return (-1);
  }
}

int MONIT_PTH_select() {
  int addr = 0x76;
  if (ioctl(i2c_file, I2C_SLAVE, addr) < 0) {
    fprintf(stderr,
            "Failed to acquire PTH bus access and/or talk to slave (with "
            "address %d).\n",
            addr);
    return (-1);
  }
  return (1);
}

int MONIT_PTH_read_id() {
  // register add
  alt_u8 addr = 0xD0;
  // write register addr
  if (write(i2c_file, &addr, 1) != 1) {
    fprintf(stderr, "Failed to write register addr. to PTH.\n");
    return (-1);
  }
  // read id
  alt_u8 id;
  if (read(i2c_file, &id, 1) != 1) {
    fprintf(stderr, "Failed to read PTH id.\n");
    return (-1);
  }
  return ((int)id);
}

int MONIT_PT100_select() {
  int addr = 0x44;
  if (ioctl(i2c_file, I2C_SLAVE, addr) < 0) {
    fprintf(stderr,
            "Failed to acquire PT100 bus access and/or talk to slave (with "
            "address %d).\n",
            addr);
    return (-1);
  }
  return (1);
}

int MONIT_PT100_write_reg(alt_u8 reg_addr, alt_u8 value) {
  // write register addr
  alt_u8 frame[2];
  frame[0] = 0x40 | ((reg_addr << 2) & 0XC);
  frame[1] = value;
  if (write(i2c_file, frame, 2) != 2) {
    fprintf(stderr, "Failed to write to PT100, reg addr=%x, value=%x\n",
            reg_addr, value);
    return (-1);
  }
  alt_u8 check;
  frame[0] = 0x20 | ((reg_addr << 2) & 0XC);
  if (write(i2c_file, frame, 1) != 1) {
    fprintf(stderr, "Failed to access to PT100 reg @ addr=%x\n", reg_addr);
    return (-1);
  }
  if (read(i2c_file, &check, 1) != 1) {
    fprintf(stderr, "Failed to read PT100 reg @ addr=%x\n", reg_addr);
    return (-1);
  }
  if (check != value) {
    fprintf(stderr, "PT100: written and read value differ.\n");
    fprintf(stderr, "read value = %x\n", check);
    return (-1);
  }
  return ((int)check);
}

int MONIT_PWRMON_select(int rfm) {
  int addr = 0x40 + (rfm & 0x3);
  if (ioctl(i2c_file, I2C_SLAVE, addr) < 0) {
    fprintf(stderr,
            "Failed to acquire INA226 (RFM no %d) bus access and/or talk to "
            "slave (with address %d).\n",
            rfm, addr);
    return (-1);
  }
  return (1);
}

int MONIT_PWRMON_readreg(alt_u8 reg_addr) {
  // write register addr
  if (write(i2c_file, &reg_addr, 1) != 1) {
    fprintf(stderr, "Failed to write register addr. to INA226\n");
    return (-1);
  }
  // read reg
  alt_u8 val[2];
  if (read(i2c_file, val, 2) != 2) {
    fprintf(stderr, "Failed to read INA226 reg %x\n", reg_addr);
    return (-1);
  }
  int result = val[0] * 0x100 + val[1];
  return (result);
}

void MONIT_BBL_reset() {
  *p_pio_ctrl |= O_BBL_RST;
  usleep(1);
  *p_pio_ctrl &= ~O_BBL_RST;
  return;
}

/*************** clocks and synchronization ************/

void SYNC_reset_fpga() {
  *p_pio_ctrl = O_RESET;
  fprintf(stderr, "Reset FPGA\n");
  usleep(1);
  *p_pio_ctrl = 0x00000000;
  usleep(1);
}

void SYNC_reset_hr() {
  *p_pio_ctrl |= O_RESET_HR;
  // fprintf(stderr,"reset hardroc\n");
  usleep(1);
  *p_pio_ctrl &= ~O_RESET_HR;
  usleep(1);
}

int SYNC_pll_lck() { return ((*p_pio_state & O_PLL_LCK) == O_PLL_LCK); }

int SYNC_state() { return (((*p_pio_state) >> SHIFT_SYNCHRO_STATE) & 0xF); }

void SYNC_select_command_soft() { *p_pio_ctrl |= O_SEL_CMD_SOURCE; }

void SYNC_select_command_dcc() { *p_pio_ctrl &= ~O_SEL_CMD_SOURCE; }

void SYNC_set_command(alt_u32 cmd) {
  alt_u32 pio_val = *p_pio_ctrl;
  pio_val &= ~(0xF << SHIFT_CMD_CODE);      // reset 4 bits
  pio_val |= (0xF & cmd) << SHIFT_CMD_CODE; // set command
  *p_pio_ctrl = pio_val;
  usleep(2);
  pio_val &= ~(0xF << SHIFT_CMD_CODE);   // reset 4 bits
  pio_val |= CMD_IDLE << SHIFT_CMD_CODE; // set idle command
  *p_pio_ctrl = pio_val;
  // alt_u32 mask = (0xF & cmd) << SHIFT_CMD_CODE;
  //*p_pio_ctrl &= ~mask; //set 0's
  //*p_pio_ctrl |= mask;  //set 1's
  // usleep(2);
  // SYNC_set_command(CMD_IDLE);
}

void SYNC_reset_bcid() { SYNC_set_command(CMD_RESET_BCID); }

void SYNC_start_acq() { SYNC_set_command(CMD_START_ACQ); }

void SYNC_stop_acq() { SYNC_set_command(CMD_STOP_ACQ); }

void SYNC_ramfull_ext() { SYNC_set_command(CMD_RAMFULL_EXT); }

void SYNC_digital_ro() { SYNC_set_command(CMD_DIGITAL_RO); }

void SYNC_fifo_arming() { *p_pio_ctrl |= O_HPS_BUSY; }

void SYNC_fifo_ack() {
  *p_pio_ctrl &= ~O_HPS_BUSY; // falling edge on hps_busy
  while (SYNC_state() != S_IDLE) {
    ;
  }                          // when FGPA ready for new acqui
  *p_pio_ctrl |= O_HPS_BUSY; // rearming
}

int SYNC_dcc_cmd_mem() { return ((*p_pio_cnt24 >> SHIFT_CMD_CODE_MEM) & 0xF); }

int SYNC_dcc_cmd_now() { return ((*p_pio_cnt24 >> SHIFT_CMD_CODE_NOW) & 0xF); }

int SYNC_ramfull() {
  int state = SYNC_state();
  if (state == S_RAMFULL)
    return (1);
  else
    return (0);
}

int SYNC_fpga_ro() {
  int state = SYNC_state();
  if ((state == S_START_RO) || (state == S_WAIT_END_RO))
    return (1);
  else
    return (0);
}

int SYNC_fifo_ready() {
  int state = SYNC_state();
  if (state == S_FIFO_READY)
    return (1);
  else
    return (0);
}

int SYNC_run_stopped() {
  int state = SYNC_state();
  if (state == S_STOP_RUN)
    return (1);
  else
    return (0);
}

int SYNC_hr_transmiton(int rfm) {
  switch (rfm) {
  case 0:
    return ((*p_pio_state & O_HR_TRANSMITON_0) == O_HR_TRANSMITON_0);
  case 1:
    return ((*p_pio_state & O_HR_TRANSMITON_1) == O_HR_TRANSMITON_1);
  case 2:
    return ((*p_pio_state & O_HR_TRANSMITON_2) == O_HR_TRANSMITON_2);
  case 3:
    return ((*p_pio_state & O_HR_TRANSMITON_3) == O_HR_TRANSMITON_3);
  default:
    fprintf(stderr, "hr_transmiton: bad argument %d\n", rfm);
    return (-1);
  }
}

int SYNC_chipsat(int rfm) {
  switch (rfm) {
  case 0:
    return ((*p_pio_state & O_CHIPSAT_0) == O_CHIPSAT_0);
  case 1:
    return ((*p_pio_state & O_CHIPSAT_1) == O_CHIPSAT_1);
  case 2:
    return ((*p_pio_state & O_CHIPSAT_2) == O_CHIPSAT_2);
  case 3:
    return ((*p_pio_state & O_CHIPSAT_3) == O_CHIPSAT_3);
  default:
    fprintf(stderr, "chipsat: bad argument %d\n", rfm);
    return (-1);
  }
}

int SYNC_hr_end_ro(int rfm) {
  switch (rfm) {
  case 0:
    return ((*p_pio_state & O_HR_END_RO_0) == O_HR_END_RO_0);
  case 1:
    return ((*p_pio_state & O_HR_END_RO_1) == O_HR_END_RO_1);
  case 2:
    return ((*p_pio_state & O_HR_END_RO_2) == O_HR_END_RO_2);
  case 3:
    return ((*p_pio_state & O_HR_END_RO_3) == O_HR_END_RO_3);
  default:
    fprintf(stderr, "hr_end_ro: bad argument %d\n", rfm);
    return (-1);
  }
}

void SYNC_enable_dcc_busy() { *p_pio_ctrl |= O_ENA_DCC_BUSY; }

void SYNC_enable_dcc_ramfull() { *p_pio_ctrl |= O_ENA_DCC_RAMFULL; }

/*************** trigger configuration *****************/

void TRIG_select_thresh_0() { *p_pio_ctrl &= ~O_SEL_TRIG_THRESH; };

void TRIG_select_thresh_1() { *p_pio_ctrl |= O_SEL_TRIG_THRESH; };

void TRIG_enable() { *p_pio_ctrl |= O_ENA_TRIG; };

void TRIG_disable() { *p_pio_ctrl &= ~O_ENA_TRIG; };

/*************** counters (time, events) ***************/

void CNT_reset() {
  *p_pio_ctrl |= O_RST_SCALERS;
  // fprintf(stderr,"reset scaler\n");
  usleep(1);
  *p_pio_ctrl &= ~O_RST_SCALERS;
}

void CNT_start() { *p_pio_ctrl |= O_ENA_SCALERS; }

void CNT_stop() { *p_pio_ctrl &= ~O_ENA_SCALERS; }

alt_u32 CNT_hit0(int rfm) { return (*p_pio_cnt_hit0[rfm]); }

alt_u32 CNT_hit1(int rfm) { return (*p_pio_cnt_hit1[rfm]); }

alt_u32 CNT_trig() { return (*p_pio_cnt_trig); }

alt_u32 CNT_bcid24() { return (*p_pio_cnt24 & 0xFFFFFF); }

alt_u32 CNT_bcid48msb() { return (*p_pio_cnt48_msb & 0xFFFF); }

alt_u32 CNT_bcid48lsb() { return (*p_pio_cnt48_lsb); }

// write counters in a file
// format: 7 x 32-bit words per ramfull-trigger
void CNT_save(FILE *output, int rfm) {
  alt_u32 word32;
  alt_u32 dif_id = EDA_DIF_ID_OFFS + ((this_eda_id & 7) << 3) + (rfm & 3);
  word32 = daq_cycle_id[rfm];
  word32 = (dif_id << 24) | (word32 & 0x00FFFFFF);
  fwrite_u32(word32, output);
  fwrite_u32(CNT_bcid48msb(), output);
  fwrite_u32(CNT_bcid48lsb(), output);
  fwrite_u32(CNT_bcid24(), output);
  fwrite_u32(CNT_hit0(rfm), output);
  fwrite_u32(CNT_hit1(rfm), output);
  fwrite_u32(CNT_trig(), output);
  return;
}

/****************** Hardroc slow control **************/

void HRSC_select_slow_control() { *p_pio_ctrl &= ~O_SELECT_SC_RR; }

void HRSC_select_read_register() { *p_pio_ctrl |= O_SELECT_SC_RR; }

void HRSC_set_bit(alt_u32 hr_addr, alt_u32 bit_addr, alt_u32 bit) {
  // byte address 0 corresponds to the last register (bit_addr 864 to 871) of
  // the last Hardroc (pos=NB_HR-1)
  div_t frac = div(bit_addr, 8);
  alt_u32 byte_addr =
      (NB_HR - 1 - hr_addr) * NB_BYTES_CFG_HR + NB_BYTES_CFG_HR - 1 - frac.quot;
  uint8_t byte = hr_config_data[byte_addr];
  alt_u32 bit_offset = frac.rem;
  // bit address increases from lsb to msb
  uint8_t mask1 = 0x01 << bit_offset;
  uint8_t mask2 = (0x01 & bit) << bit_offset;
  byte &= ~mask1; // reset target bit
  byte |= mask2;  // set target bit = "bit" argument
  hr_config_data[byte_addr] = byte;
  return;
}

alt_u32 HRSC_get_bit(alt_u32 hr_addr, alt_u32 bit_addr) {
  // byte address 0 corresponds to the last register (bit_addr 864-871) of the
  // last Hardroc (pos=NB_HR-1)
  div_t frac = div(bit_addr, 8);
  alt_u32 byte_addr =
      (NB_HR - 1 - hr_addr) * NB_BYTES_CFG_HR + NB_BYTES_CFG_HR - 1 - frac.quot;
  uint8_t byte = hr_config_data[byte_addr];
  alt_u32 bit_offset = frac.rem;
  return ((byte >> bit_offset) & 0x01);
}

void HRSC_set_word(alt_u32 hr_addr, alt_u32 bit_addr, alt_u32 n_bits,
                   alt_u32 value) {
  int i;
  for (i = 0; i < n_bits; i++) { // scan lsb to msb
    alt_u32 bit = (value >> i) & 0x01;
    HRSC_set_bit(hr_addr, bit_addr + i, bit);
  }
  return;
}

void HRSC_set_word_msb2lsb(alt_u32 hr_addr, alt_u32 bit_addr, alt_u32 n_bits,
                           alt_u32 value) {
  int i;
  for (i = 0; i < n_bits; i++) { // scan msb to lsb
    alt_u32 bit = (value >> i) & 0x01;
    HRSC_set_bit(hr_addr, bit_addr + n_bits - 1 - i, bit);
  }
  return;
}

int HRSC_read_conf_singl(FILE *file_conf, alt_u32 hr_addr) {
  ssize_t n = 0;
  alt_u32 bit_addr, bit;
  alt_u32 cnt = NB_BITS_CFG_HR - 1;
  int delimiter = ';';
  char *str = NULL;
  size_t len = 0;
  // printf("bit_addr\tbit_value\n");
  while (!feof(file_conf)) {
    // bit addr
    n = getdelim(&str, &len, delimiter, file_conf);
    if (n == 0) {
      continue;
    }
    if (str[0] == '#') { // if comment line, skip
      n = getline(&str, &len, file_conf);
      if (n == 0) {
        continue;
      }
      n = getdelim(&str, &len, delimiter, file_conf);
      if (n == 0) {
        continue;
      }
    }
    bit_addr = atoi(str);
    // skip reg name and indices
    n = getdelim(&str, &len, delimiter, file_conf);
    if (n == 0) {
      continue;
    }
    n = getdelim(&str, &len, delimiter, file_conf);
    if (n == 0) {
      continue;
    }
    n = getdelim(&str, &len, delimiter, file_conf);
    if (n == 0) {
      continue;
    }
    // bit value
    n = getline(&str, &len, file_conf);
    if (n == 0) {
      continue;
    }
    bit = atoi(str);
    // check bit address
    if (bit_addr != cnt) {
      printf("config file error: bit address %u expected, %s found\n",
             (uint32_t)cnt, str);
      return (-1);
    }
    cnt--;
    // set bit
    // printf("%u\t%u\n",bit_addr,bit);
    HRSC_set_bit(hr_addr, bit_addr, bit);
    if (bit_addr == 0)
      return (1);
  }
  printf("config file error: end of file found before last bit\n");
  return (-1);
}

void HRSC_copy_conf(alt_u32 hr_addr_source, alt_u32 hr_addr_dest) {
  alt_u32 source_addr = (NB_HR - 1 - hr_addr_source) * NB_BYTES_CFG_HR;
  alt_u32 dest_addr = (NB_HR - 1 - hr_addr_dest) * NB_BYTES_CFG_HR;
  memcpy(hr_config_data + dest_addr, hr_config_data + source_addr,
         NB_BYTES_CFG_HR);
  return;
}

int HRSC_read_conf_mult(FILE *file_conf) {
  ssize_t n = 0;
  alt_u32 hr_addr, bit_addr, bit;
  alt_u32 cnt_bit = NB_BITS_CFG_HR - 1;
  alt_u32 cnt_hr = NB_HR - 1;
  int delimiter = ';';
  char *str = NULL;
  size_t len = 0;
  while (!feof(file_conf) && (cnt_bit >= 0) && (cnt_hr >= 0)) {
    // hr_addr
    n = getdelim(&str, &len, delimiter, file_conf);
    if (n == 0) {
      continue;
    }
    hr_addr = atoi(str);
    // bit addr
    n = getdelim(&str, &len, delimiter, file_conf);
    if (n == 0) {
      continue;
    }
    bit_addr = atoi(str);
    // bit value
    n = getline(&str, &len, file_conf);
    if (n == 0) {
      continue;
    }
    bit = atoi(str);
    // check hr and bit addresses
    if (bit_addr != cnt_bit) {
      printf("format error: bit address %u expected, %u found\n",
             (uint32_t)cnt_bit, (uint32_t)bit_addr);
      return (-1);
    }
    if (hr_addr != cnt_hr) {
      printf("format error: HR address %u expected, %u found\n",
             (uint32_t)cnt_hr, (uint32_t)hr_addr);
      return (-1);
    }
    cnt_bit--;
    if (cnt_bit == 0) {
      cnt_bit = NB_BITS_CFG_HR - 1;
      cnt_hr--;
    }
    // set bit
    HRSC_set_bit(hr_addr, bit_addr, bit);
    if ((bit_addr == 0) && (hr_addr == 0))
      return (1);
  }
  printf("config file error: end of file found before last bit\n");
  return (-1);
}

int HRSC_write_conf_mult(FILE *file_conf) {
  alt_u32 i, j;
  alt_u32 cnt_bit, cnt_hr;
  for (i = 0; i < NB_HR; i++) {
    for (j = 0; j < NB_BITS_CFG_HR; j++) {
      cnt_hr = NB_HR - 1 - i;
      cnt_bit = NB_BITS_CFG_HR - 1 - j;
      fprintf(file_conf, "%u;%u;%u\n", (uint32_t)cnt_hr, (uint32_t)cnt_bit,
              (uint32_t)HRSC_get_bit(cnt_hr, cnt_bit));
    }
  }
  return (1);
}

void HRSC_set_ctest(alt_u32 hr_addr, alt_u32 chan,
                    alt_u32 val) { // switch for test capacitor (1=closed)
  HRSC_set_bit(hr_addr, chan, (val & 0x01));
}

void HRSC_set_all_ctest_off() {
  alt_u32 hr_addr, chan;
  for (hr_addr = 0; hr_addr < NB_HR; hr_addr++)
    for (chan = 0; chan < 64; chan++)
      HRSC_set_ctest(hr_addr, chan, 0);
}

void HRSC_set_preamp(alt_u32 hr_addr, alt_u32 chan, alt_u32 val) {
  alt_u32 bit_addr = 64 + 8 * chan;
  HRSC_set_word(hr_addr, bit_addr, 8, val);
}

void HRSC_set_cmd_fsb2(alt_u32 hr_addr, alt_u32 chan,
                       alt_u32 val) { // fast shaper 2 gain
  alt_u32 bit_addr = 587 + 4 * chan;
  HRSC_set_word_msb2lsb(hr_addr, bit_addr, 4,
                        (~val)); //"cmdb" register bits are active-low
}

void HRSC_set_cmd_fsb1(alt_u32 hr_addr, alt_u32 chan,
                       alt_u32 val) { // fast shaper 2 gain
  alt_u32 bit_addr = 595 + 4 * chan;
  HRSC_set_word_msb2lsb(hr_addr, bit_addr, 4,
                        (~val)); //"cmdb" register bits are active-low
}

void HRSC_set_mask(alt_u32 hr_addr, alt_u32 chan, alt_u32 val) {
  alt_u32 bit_addr = 618 + 3 * chan;
  HRSC_set_word(hr_addr, bit_addr, 3, val);
}

void HRSC_set_chip_id(alt_u32 hr_addr, alt_u32 val) {
  HRSC_set_word_msb2lsb(hr_addr, 810, 8, val);
}

void HRSC_set_DAC0(alt_u32 hr_addr, alt_u32 val) {
  HRSC_set_word(hr_addr, 818, 10, val);
}

void HRSC_set_DAC1(alt_u32 hr_addr, alt_u32 val) {
  HRSC_set_word(hr_addr, 828, 10, val);
}

void HRSC_set_DAC2(alt_u32 hr_addr, alt_u32 val) {
  HRSC_set_word(hr_addr, 838, 10, val);
}

void HRSC_set_DAC_coarse(alt_u32 hr_addr) { HRSC_set_bit(hr_addr, 848, 0); }

void HRSC_set_DAC_fine(alt_u32 hr_addr) { HRSC_set_bit(hr_addr, 848, 1); }

void HRSC_set_shaper_capa(alt_u32 hr_addr, alt_u32 val) {
  HRSC_set_bit(hr_addr, 611, (val & 1));        // sw_50f0 = b0
  HRSC_set_bit(hr_addr, 602, (val & 1));        // sw_50f1 = b0
  HRSC_set_bit(hr_addr, 594, (val & 1));        // sw_50f2 = b0
  HRSC_set_bit(hr_addr, 610, ((val >> 1) & 1)); // sw_100f0 = b1
  HRSC_set_bit(hr_addr, 601, ((val >> 1) & 1)); // sw_100f1 = b1
  HRSC_set_bit(hr_addr, 593, ((val >> 1) & 1)); // sw_100f2 = b1
}

void HRSC_set_shaper_resis(alt_u32 hr_addr, alt_u32 val) {
  HRSC_set_bit(hr_addr, 609, (val & 1));        // sw_100k0 = b0
  HRSC_set_bit(hr_addr, 600, (val & 1));        // sw_100k1 = b0
  HRSC_set_bit(hr_addr, 592, (val & 1));        // sw_100k2 = b0
  HRSC_set_bit(hr_addr, 608, ((val >> 1) & 1)); // sw_50k0 = b1
  HRSC_set_bit(hr_addr, 599, ((val >> 1) & 1)); // sw_50k1 = b1
  HRSC_set_bit(hr_addr, 591, ((val >> 1) & 1)); // sw_50k2 = b1
}

int HRSC_set_config(int rfm) {
  // change the control word
  alt_u32 checkword;
  if ((*p_pio_sc_check[rfm]) == 0xCAFEFADE)
    checkword = 0x36BAFFE5;
  else
    checkword = 0xCAFEFADE;
  hr_config_buffer[0] = (checkword >> 24) & 0xFF;
  hr_config_buffer[1] = (checkword >> 16) & 0xFF;
  hr_config_buffer[2] = (checkword >> 8) & 0xFF;
  hr_config_buffer[3] = checkword & 0xFF;
  // reset sc
  HRSC_select_slow_control();
  HRSC_reset_sc();
  if (HRSC_sc_done(rfm)) {
    fprintf(stderr, "error: slow control reset failed\n");
    return (-1);
  }
  // copy to fpga
  void *dest = (void *)p_ram_sc[rfm];
  memcpy(dest, hr_config_buffer, NB_HR * NB_BYTES_CFG_HR + 4);
  // trigger the slow control serializer
  HRSC_start_sc(rfm);
  // check loopback header
  usleep(10);
  while (!HRSC_sc_done(rfm))
    usleep(10);
  if (*p_pio_sc_check[rfm] != checkword) {
    fprintf(stderr, "loopback register=%x\n", (uint32_t)(*p_pio_sc_check[rfm]));
    return (-1);
  }
  return (1);
}

int HRSC_reset_read_registers(int rfm) {
  // change the control word
  alt_u32 checkword;
  if ((*p_pio_sc_check[rfm]) == 0xCAFEFADE)
    checkword = 0x36BAFFE5;
  else
    checkword = 0xCAFEFADE;
  alt_u8 buffer[NB_HR * NB_BYTES_CFG_HR + 4] = {0};
  int offset = NB_HR * NB_BYTES_CFG_HR - 64;
  buffer[offset] = (checkword >> 24) & 0xFF;
  buffer[offset + 1] = (checkword >> 16) & 0xFF;
  buffer[offset + 2] = (checkword >> 8) & 0xFF;
  buffer[offset + 3] = checkword & 0xFF;
  // reset sc
  HRSC_select_read_register();
  HRSC_reset_sc();
  if (HRSC_sc_done(rfm)) {
    fprintf(stderr, "error: slow control reset failed\n");
    return (-1);
  }
  // copy to fpga
  void *dest = (void *)p_ram_sc[rfm];
  memcpy(dest, buffer, NB_HR * NB_BYTES_CFG_HR + 4);
  // trigger the slow control serializer
  usleep(10);
  HRSC_start_sc(rfm);
  // check loopback header
  usleep(10);
  while (!HRSC_sc_done(rfm))
    usleep(10);
  if (*p_pio_sc_check[rfm] != checkword) {
    fprintf(stderr, "loopback register=%x\n", (uint32_t)(*p_pio_sc_check[rfm]));
    return (-1);
  }
  HRSC_select_slow_control();
  return (1);
}

int HRSC_set_read_register(int rfm, int channel) {
  // change the control word
  alt_u32 checkword;
  if ((*p_pio_sc_check[rfm]) == 0xCAFEFADE)
    checkword = 0x36BAFFE5;
  else
    checkword = 0xCAFEFADE;
  alt_u8 buffer[NB_HR * NB_BYTES_CFG_HR + 4] = {0};
  int offset = NB_HR * NB_BYTES_CFG_HR - 64;
  buffer[offset] = (checkword >> 24) & 0xFF;
  buffer[offset + 1] = (checkword >> 16) & 0xFF;
  buffer[offset + 2] = (checkword >> 8) & 0xFF;
  buffer[offset + 3] = checkword & 0xFF;
  // reset sc
  HRSC_select_read_register();
  HRSC_reset_sc();
  if (HRSC_sc_done(rfm)) {
    fprintf(stderr, "error: slow control reset failed\n");
    return (-1);
  }
  // select the same channel for all HR's
  div_t frac = div(channel, 8); // channel: 0 to 64*NB_HR-1
  alt_u8 byte = 0x1 << frac.rem;
  int i;
  for (i = 0; i < 8; i++) {
    offset = NB_HR * NB_BYTES_CFG_HR + 3 - i * 8 -
             frac.quot; // last byte -> channels 0 to 7
    buffer[offset] = byte;
  }
  // copy to fpga
  void *dest = (void *)p_ram_sc[rfm];
  memcpy(dest, buffer, NB_HR * NB_BYTES_CFG_HR + 4);
  // trigger the slow control serializer
  HRSC_start_sc(rfm);
  // check loopback header
  usleep(10);
  while (!HRSC_sc_done(rfm))
    usleep(10);
  if (*p_pio_sc_check[rfm] != checkword) {
    fprintf(stderr, "loopback register=%x\n", (uint32_t)(*p_pio_sc_check[rfm]));
    return (-1);
  }
  HRSC_select_slow_control();
  return (1);
}

void HRSC_reset_sc() {
  *p_pio_ctrl |= O_RESET_SC;
  // fprintf(stderr,"Reset slow control\n");
  usleep(1);
  *p_pio_ctrl &= ~O_RESET_SC;
  usleep(1);
  // fprintf(stderr,"reset_sc; sc_done=%d\n",sc_done(0x01));
}

void HRSC_start_sc(int rfm) {
  switch (rfm) {
  case 0:
    *p_pio_ctrl |= O_START_SC_0;
    usleep(1);
    *p_pio_ctrl &= ~O_START_SC_0;
    return;
  case 1:
    *p_pio_ctrl |= O_START_SC_1;
    usleep(1);
    *p_pio_ctrl &= ~O_START_SC_1;
    return;
  case 2:
    *p_pio_ctrl |= O_START_SC_2;
    usleep(1);
    *p_pio_ctrl &= ~O_START_SC_2;
    return;
  case 3:
    *p_pio_ctrl |= O_START_SC_3;
    usleep(1);
    *p_pio_ctrl &= ~O_START_SC_3;
    return;
  default:
    fprintf(stderr, "start slow control: bad argument %d\n", rfm);
    return;
  }
}

int HRSC_sc_done(int rfm) {
  switch (rfm) {
  case 0:
    return ((*p_pio_state & O_SC_DONE_0) == O_SC_DONE_0);
  case 1:
    return ((*p_pio_state & O_SC_DONE_1) == O_SC_DONE_1);
  case 2:
    return ((*p_pio_state & O_SC_DONE_2) == O_SC_DONE_2);
  case 3:
    return ((*p_pio_state & O_SC_DONE_3) == O_SC_DONE_3);
  default:
    fprintf(stderr, "sc_done: bad argument %d\n", rfm);
    return (-1);
  }
}

/********************* Hardroc DAQ *********************/

alt_u32 bit(alt_u32 word, alt_u32 digit) {
  return ((word >> digit) & MASK_1_BIT);
}

void DAQ_fifo_init(int rfm) {
  alt_u32 *fifo_csr = p_fifo_daq_csr[rfm];
  fifo_csr[ALTERA_AVALON_FIFO_EVENT_REG] =
      ALTERA_AVALON_FIFO_EVENT_ALL; // clear event reg (write 1 to each field)
  fifo_csr[ALTERA_AVALON_FIFO_IENABLE_REG] = 0; // disable interrupts
  fifo_csr[ALTERA_AVALON_FIFO_ALMOSTFULL_REG] =
      (alt_u32)5081; // set "almostfull" to (max_size + 1)
  fifo_csr[ALTERA_AVALON_FIFO_ALMOSTEMPTY_REG] = (alt_u32)2; // set
                                                             // "almostempty"
  return;
}

void DAQ_fifo_print_status(int rfm) { // print status register
  alt_u32 *fifo_csr = p_fifo_daq_csr[rfm];
  fprintf(stderr, "fifo/status_reg=%04x\n",
          (uint32_t)fifo_csr[ALTERA_AVALON_FIFO_STATUS_REG]);
  return;
}

void DAQ_fifo_print_event(int rfm) { // print event register
  alt_u32 *fifo_csr = p_fifo_daq_csr[rfm];
  fprintf(stderr, "fifo/event_reg=%04x\n",
          (uint32_t)fifo_csr[ALTERA_AVALON_FIFO_EVENT_REG]);
  return;
}

void DAQ_fifo_clear_event(int rfm) { // clear event register
  alt_u32 *fifo_csr = p_fifo_daq_csr[rfm];
  fifo_csr[ALTERA_AVALON_FIFO_EVENT_REG] =
      ALTERA_AVALON_FIFO_EVENT_ALL; // clear event reg (write 1 to each field)
  return;
}

int DAQ_fifo_in_valid(int rfm) {
  switch (rfm) {
  case 0:
    return ((*p_pio_state & O_FIFO_IN_VALID_0) == O_FIFO_IN_VALID_0);
  case 1:
    return ((*p_pio_state & O_FIFO_IN_VALID_1) == O_FIFO_IN_VALID_1);
  case 2:
    return ((*p_pio_state & O_FIFO_IN_VALID_2) == O_FIFO_IN_VALID_2);
  case 3:
    return ((*p_pio_state & O_FIFO_IN_VALID_3) == O_FIFO_IN_VALID_3);
  default:
    fprintf(stderr, "fifo_in_valid: bad argument %d\n", rfm);
    return (-1);
  }
}

int DAQ_fifo_full(int rfm) {
  alt_u32 istatus = p_fifo_daq_csr[rfm][ALTERA_AVALON_FIFO_STATUS_REG];
  return (bit(istatus, 0));
}

int DAQ_fifo_empty(int rfm) {
  alt_u32 istatus = p_fifo_daq_csr[rfm][ALTERA_AVALON_FIFO_STATUS_REG];
  return (bit(istatus, 1));
}

alt_u32 DAQ_fifo_fill_level(int rfm) {
  return p_fifo_daq_csr[rfm][ALTERA_AVALON_FIFO_LEVEL_REG];
}

alt_u32 DAQ_fifo_data(int rfm) {
  return (p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG]);
}

alt_u32 garbage;
alt_u32 DAQ_fifo_clear(int rfm) {
  alt_u32 count_words = 0;
  while (!DAQ_fifo_empty(rfm)) {
    garbage = p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG];
    count_words++;
  }
  return (count_words / 5);
}

// a simple binary format for debug mode:
// 6 x 32-bit words per event
//      1: dif_id;dif_trigger_counter
//      2: hr_id;bcid
//      3-6 : discri data

// note on DAQ architecture: each RFM & chamber is identified by a DIF id
// and EDA produces a data stream for each RFM.
// DIF id's for new electronics are under 100 (above 100 is for old DIFs)

alt_u32
DAQ_save_hr_data(FILE *output,
                 int rfm) { // fill the buffer and return the nb of events read
  int cnt = 0;
  int i;
  alt_u32 word32, dif_id;
  // DIF id
  dif_id = EDA_DIF_ID_OFFS + ((this_eda_id & 7) << 3) + (rfm & 3);
  while (!DAQ_fifo_empty(rfm)) {
    word32 = (dif_id << 24) | (daq_cycle_id[rfm] & 0x00FFFFFF);
    fwrite_u32(word32, output);
    for (i = 0; i < 5; i++) {
      fwrite_u32((p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG]), output);
      cnt++;
    }
  }
  daq_cycle_id[rfm]++;
  return (cnt / 5);
}

alt_u32 DAQ_hexdump_hr_data(
    FILE *output, int rfm) { // fill the buffer and return the nb of events read
  int cnt = 0;
  int i;
  alt_u32 word32, dif_id;
  // DIF id
  dif_id = EDA_DIF_ID_OFFS + ((this_eda_id & 7) << 3) + (rfm & 3);
  while (!DAQ_fifo_empty(rfm)) {
    word32 = (dif_id << 24) | (daq_cycle_id[rfm] & 0x00FFFFFF);
    fprintf(output, "%08x\t", (uint32_t)word32);
    for (i = 0; i < 5; i++) {
      fprintf(output, "%08x\t",
              (uint32_t)(p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG]));
      cnt++;
    }
    fprintf(output, "\n");
  }
  daq_cycle_id[rfm]++;
  return (cnt / 5);
}

alt_u32 DAQ_save_hr_data_DIF(
    FILE *output, int rfm) { // fill the buffer and return the nb of events read
  int cnt = 0;
  int i, nb_ram_units;
  alt_u8 word8;
  alt_u16 word16;
  alt_u32 word32;
  alt_u64 bcid48;
  alt_u32 bcid24;

  // DIF DAQ header
  fwrite_u8((alt_u8)0xB0, output);
  // DIF id
  word8 = EDA_DIF_ID_OFFS + ((this_eda_id & 7) << 3) + (rfm & 3);
  fwrite_u8(word8, output);
  // counters
  fwrite_u32(daq_cycle_id[rfm], output);
  fwrite_u32(CNT_hit0(rfm), output);
  fwrite_u32(CNT_hit1(rfm), output);
  word16 = CNT_bcid48msb(rfm) & 0xFFFF;
  bcid48 = word16;
  fwrite_u16(word16, output);
  word32 = CNT_bcid48lsb(rfm);
  fwrite_u32(word32, output);
  bcid48 = (bcid48 << 32) | word32;
  bcid24 = CNT_bcid24(rfm);
  word8 = bcid24 >> 16;
  word16 = bcid24 & 0xFFFF;
  fwrite_u8(word8, output);
  fwrite_u16(word16, output);
  // unused "nb_lines"
  fwrite_u8(0xFF, output);
  // HR DAQ chunk
  int last_hr_id = -1;
  int hr_id;
  while (!DAQ_fifo_empty(rfm)) {
    // read HR id
    word32 = p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG];
    hr_id = (int)(word32 >> 24);
    // if new HR id, insert (trailer and) header
    if (hr_id != last_hr_id) {
      if (last_hr_id >= 0)
        fwrite_u8((alt_u8)0xA3, output); // hr trailer
      fwrite_u8((alt_u8)0xB4, output);   // hr header
    }
    fwrite_u32(word32, output);
    for (i = 0; i < 4; i++) {
      fwrite_u32((p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG]), output);
      cnt++;
    }
    last_hr_id = hr_id;
  }
  fwrite_u8((alt_u8)0xA3, output);     // last hr trailer
  fwrite_u8((alt_u8)0xA0, output);     // DIF DAQ trailer
  fwrite_u16((alt_u16)0xC0C0, output); // CRC
  // on-line monit
  nb_ram_units = cnt / 5;
  // fprintf(stderr,"%lu\t%llu\t%lu\t%d\t%.3e\n",daq_cycle_id[rfm],bcid48,bcid24,nb_ram_units,ram_rate);

  daq_cycle_id[rfm]++;
  return (nb_ram_units);
}

alt_u8 *buf_wr_8(alt_u8 *buf, alt_u8 val) {
  *buf = val;
  return (buf + 1);
}

alt_u8 *buf_wr_16(alt_u8 *buf, alt_u16 val) {
  alt_u16 bigend = htons(val);
  memcpy(buf, &bigend, 2);
  return (buf + 2);
}

alt_u8 *buf_wr_32(alt_u8 *buf, alt_u32 val) {
  alt_u32 bigend = htonl(val);
  memcpy(buf, &bigend, 4);
  return (buf + 4);
}

void DAQ_bufferize_data_DIF(int rfm) { // fill a buffer with RFM data
  int i;
  static alt_u32 bcid48_offset;
  alt_u8 word8;
  alt_u16 word16;
  alt_u32 word32;
  alt_u64 bcid48;
  alt_u32 bcid24;

  // offset
  if (daq_cycle_id[rfm] == 0)
    bcid48_offset = CNT_bcid48lsb(rfm) - CNT_bcid24(rfm);
  // NB: at this time, CNT_bcid48msb(rfm)==0
  // DIF DAQ header
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xB0);
  // DIF id
  word8 = EDA_DIF_ID_OFFS + ((this_eda_id & 7) << 3) + (rfm & 3);
  hr_data_cursor = buf_wr_8(hr_data_cursor, word8);
  // counters
  hr_data_cursor = buf_wr_32(hr_data_cursor, daq_cycle_id[rfm]);
  hr_data_cursor = buf_wr_32(hr_data_cursor, CNT_hit0(rfm));
  hr_data_cursor = buf_wr_32(hr_data_cursor, CNT_hit1(rfm));
  // assemble and correct absolute BCID
  bcid48 = CNT_bcid48msb(rfm);
  bcid48 <<= 32;
  bcid48 |= CNT_bcid48lsb(rfm);
  bcid48 -= bcid48_offset;
  // copy frame
  word16 = (bcid48 >> 32) & 0xFFFF;
  word32 = (alt_u32)bcid48; // truncates
  hr_data_cursor = buf_wr_16(hr_data_cursor, word16);
  hr_data_cursor = buf_wr_32(hr_data_cursor, word32);
  bcid24 = CNT_bcid24(rfm);
  word8 = bcid24 >> 16;
  word16 = bcid24 & 0xFFFF;
  hr_data_cursor = buf_wr_8(hr_data_cursor, word8);
  hr_data_cursor = buf_wr_16(hr_data_cursor, word16);
  // unused "nb_lines"
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xFF);
  // HR DAQ chunk
  int last_hr_id = -1;
  int hr_id;
  while (!DAQ_fifo_empty(rfm)) {
    // read HR id
    word32 = p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG];
    hr_id = (int)(word32 >> 24);
    // if new HR id, insert (trailer and) header
    if (hr_id != last_hr_id) {
      if (last_hr_id >= 0) { // hr trailer
        hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xA3);
      }
      hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xB4); // hr header
    }
    hr_data_cursor = buf_wr_32(hr_data_cursor, word32);
    for (i = 0; i < 4; i++) {
      hr_data_cursor = buf_wr_32(hr_data_cursor,
                                 p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG]);
    }
    last_hr_id = hr_id;
  }
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xA3); // last hr trailer
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xA0); // DIF DAQ trailer
  hr_data_cursor = buf_wr_16(hr_data_cursor, (alt_u16)0xC0C0); // CRC

  daq_cycle_id[rfm]++;

  return;
}

int DAQ_send_sock_buffer(int sock) {
  alt_u32 size = (alt_u32)(hr_data_cursor - hr_data_buffer);
  char sock_read_buf[4] = {0};
  int valread;
  // wait request
  valread = read(sock, sock_read_buf, 3);
  if ((valread != 3) || (strcmp(sock_read_buf, "PUL") != 0)) {
    fprintf(stderr, "instead of PUL, received :%s\n", sock_read_buf);
    return 0;
  }
  // send buffer size (32 bits little endian)
  int byte_weight;
  for (byte_weight = 0; byte_weight < 4; byte_weight++) {
    send(sock, &size, 1, 0);
    size >>= 8;
  }
  // send buffer content
  send(sock, hr_data_buffer, size, 0);
  // wait ACK
  valread = read(sock, sock_read_buf, 3);
  if ((valread != 3) || (strcmp(sock_read_buf, "ACK") != 0)) {
    fprintf(stderr, "instead of ACK, received :%s\n", sock_read_buf);
    return 0;
  }
  // rewind write pointer
  hr_data_cursor = hr_data_buffer;
  return 1;
}

int DAQ_write_buffer(FILE *f) {
  ssize_t size = (hr_data_cursor - hr_data_buffer);
  fwrite(hr_data_buffer, 1, size, f);
  // rewind write pointer
  hr_data_cursor = hr_data_buffer;
  return 1;
}

void DAQ_reset_buffer() {
  // rewind write pointer
  hr_data_cursor = hr_data_buffer;
  return;
}

/********************* Test injector *********************/

int INJ_dac_select(int rfm) {
  int addr = 0b00100000 + (rfm & 0x3);
  if (ioctl(i2c_file, I2C_SLAVE, addr) < 0) {
    fprintf(stderr,
            "Failed to acquire DAC bus access and/or talk to slave (with "
            "address %d).\n",
            addr);
    return (-1);
  }
  return (1);
}

int INJ_dac_set(alt_u32 value) {
  alt_u8 frame[3];
  // command byte
  frame[0] = 0;
  // lsb
  frame[1] = (alt_u8)(value % 0x100);
  // msb
  frame[2] = (alt_u8)(value >> 8);
  if (write(i2c_file, frame, 3) != 3) {
    fprintf(stderr, "Failed to write 3 bytes to LTC1668 DAC.\n");
    return (-1);
  }
  usleep(200); // for stabilisation
  return (1);
}

void INJ_start_ctest_pulser(alt_u32 half_period, alt_u32 phase) {
  // half period unit = 200 ns
  // phase unit = 6.25 ns
  alt_u32 word32 = ((phase & 0x1F) << 16) | (half_period & 0xFFFF);
  *p_pio_pulser = word32; // set timing registers
  usleep(1);
  *p_pio_ctrl |= O_CTEST; // enable
}

void INJ_stop_ctest_pulser() { *p_pio_ctrl &= ~O_CTEST; }

/********************** Printing  **********************/

void PRINT_fifo_status(int rfm) {
  alt_u32 *fifo_csr = p_fifo_daq_csr[rfm];
  fprintf(stderr, "----fifo status----------\n");
  fprintf(stderr, "fill level:\t\t%lu\n",
          fifo_csr[ALTERA_AVALON_FIFO_LEVEL_REG]);
  alt_u32 istatus = fifo_csr[ALTERA_AVALON_FIFO_STATUS_REG];
  fprintf(stderr, "istatus:");
  fprintf(stderr, "\t full:\t %lu", bit(istatus, 0));
  fprintf(stderr, "\t empty:\t %lu", bit(istatus, 1));
  fprintf(stderr, "\t almost full:\t %lu", bit(istatus, 2));
  fprintf(stderr, "\t almost empty:\t %lu", bit(istatus, 3));
  fprintf(stderr, "\t overflow:\t %lu", bit(istatus, 4));
  fprintf(stderr, "\t underflow:\t %lu\n", bit(istatus, 5));
  alt_u32 event = fifo_csr[ALTERA_AVALON_FIFO_EVENT_REG];
  fprintf(stderr, "event:  ");
  fprintf(stderr, "\t full:\t %lu", bit(event, 0));
  fprintf(stderr, "\t empty:\t %lu", bit(event, 1));
  fprintf(stderr, "\t almost full:\t %lu", bit(event, 2));
  fprintf(stderr, "\t almost empty:\t %lu", bit(event, 3));
  fprintf(stderr, "\t overflow:\t %lu", bit(event, 4));
  fprintf(stderr, "\t underflow:\t %lu\n", bit(event, 5));
  alt_u32 ienable = fifo_csr[ALTERA_AVALON_FIFO_IENABLE_REG];
  fprintf(stderr, "ienable:");
  fprintf(stderr, "\t full:\t %lu", bit(ienable, 0));
  fprintf(stderr, "\t empty:\t %lu", bit(ienable, 1));
  fprintf(stderr, "\t almost full:\t %lu", bit(ienable, 2));
  fprintf(stderr, "\t almost empty:\t %lu", bit(ienable, 3));
  fprintf(stderr, "\t overflow:\t %lu", bit(ienable, 4));
  fprintf(stderr, "\t underflow:\t %lu\n", bit(ienable, 5));
  fprintf(stderr, "almostfull:\t\t%lu\n",
          fifo_csr[ALTERA_AVALON_FIFO_ALMOSTFULL_REG]);
  fprintf(stderr, "almostempty:\t\t%lu\n",
          fifo_csr[ALTERA_AVALON_FIFO_ALMOSTEMPTY_REG]);
  fprintf(stderr, "\n\n");
  return;
}

void PRINT_counters(FILE *output, int rfm) {
  fprintf(output, "<counters>\n");
  fprintf(output, "#cycle_id;cnt_hit0;cnt_hit1;trig;");
  fprintf(output, "cnt48_msb;cnt48_lsb;cnt24\n");
  fprintf(output, "%u;%u;%u;%u;", (uint32_t)daq_cycle_id[rfm],
          (uint32_t)CNT_hit0(rfm), (uint32_t)CNT_hit1(rfm),
          (uint32_t)CNT_trig());
  fprintf(output, "%u;%u;%u\n", (uint32_t)CNT_bcid48msb(),
          (uint32_t)CNT_bcid48lsb(), (uint32_t)CNT_bcid24());
  return;
}

void PRINT_config(FILE *file_conf, int rfm) {
  int i, j;
  alt_u8 *p_ram = (alt_u8 *)&(p_ram_sc[rfm][0]);
  for (i = 0; i < NB_HR * NB_BYTES_CFG_HR + 4; i++) {
    j = 8 * (NB_HR * NB_BYTES_CFG_HR - i - 1);
    fprintf(file_conf, "%d\t%x\n", j, p_ram[i]);
  }
  return;
}

void PRINT_dump_registers(FILE *output) {
  fprintf(output, "p_pio_state=       %08lx\n", *p_pio_state);
  fprintf(output, "p_pio_ctrl=        %08lx\n", *p_pio_ctrl);
  fprintf(output, "p_pio_pulser=      %08lx\n", *p_pio_pulser);

  fprintf(output, "p_pio_cnt_hit0[0]= %08lx\n", *p_pio_cnt_hit0[0]);
  fprintf(output, "p_pio_cnt_hit0[1]= %08lx\n", *p_pio_cnt_hit0[1]);
  fprintf(output, "p_pio_cnt_hit0[2]= %08lx\n", *p_pio_cnt_hit0[2]);
  fprintf(output, "p_pio_cnt_hit0[3]= %08lx\n", *p_pio_cnt_hit0[3]);

  fprintf(output, "p_pio_cnt_hit1[0]= %08lx\n", *p_pio_cnt_hit1[0]);
  fprintf(output, "p_pio_cnt_hit1[1]= %08lx\n", *p_pio_cnt_hit1[1]);
  fprintf(output, "p_pio_cnt_hit1[2]= %08lx\n", *p_pio_cnt_hit1[2]);
  fprintf(output, "p_pio_cnt_hit1[3]= %08lx\n", *p_pio_cnt_hit1[3]);

  fprintf(output, "p_pio_cnt_trig=    %08lx\n", *p_pio_cnt_trig);
  fprintf(output, "p_pio_cnt48_msb=   %08lx\n", *p_pio_cnt48_msb);
  fprintf(output, "p_pio_cnt48_lsb=   %08lx\n", *p_pio_cnt48_lsb);

  fprintf(output, "p_fifo_daq_csr[0]= %08lx\n", *p_fifo_daq_csr[0]);
  fprintf(output, "p_fifo_daq_csr[1]= %08lx\n", *p_fifo_daq_csr[1]);
  fprintf(output, "p_fifo_daq_csr[2]= %08lx\n", *p_fifo_daq_csr[2]);
  fprintf(output, "p_fifo_daq_csr[3]= %08lx\n", *p_fifo_daq_csr[3]);

  char *statename[9];
  statename[0] = "idle";
  statename[1] = "reset_cnt48";
  statename[2] = "reset_cnt24";
  statename[3] = "acquiring";
  statename[4] = "ramfull";
  statename[5] = "start readout";
  statename[6] = "wait end_readout";
  statename[7] = "fifo ready";
  statename[8] = "stop run";
  int state = SYNC_state();
  fprintf(output, "synchro FSM state = %d:%s\n", state, statename[state]);

  return;
}

void device_daq_write_dif(Device_t *ctx, int slot) {
  int i;
  int rfm = slot;
  alt_u8 word8;
  alt_u16 word16;
  alt_u32 word32;
  alt_u64 bcid48;
  alt_u32 bcid24;

  ctx->task[rfm].beg = hr_data_cursor;
  // offset
  if (daq_cycle_id[rfm] == 0) {
    ctx->task[rfm].bcid48_offset = CNT_bcid48lsb(rfm) - CNT_bcid24(rfm);
  }

  // NB: at this time, CNT_bcid48msb(rfm)==0
  // DIF DAQ header
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xB0);
  // DIF id
  word8 = ctx->task[slot]
              .dif; // EDA_DIF_ID_OFFS + ((this_eda_id & 7) << 3) + (rfm & 3);
  hr_data_cursor = buf_wr_8(hr_data_cursor, word8);
  // counters
  hr_data_cursor = buf_wr_32(hr_data_cursor, daq_cycle_id[rfm]);
  hr_data_cursor = buf_wr_32(hr_data_cursor, CNT_hit0(rfm));
  hr_data_cursor = buf_wr_32(hr_data_cursor, CNT_hit1(rfm));
  // assemble and correct absolute BCID
  bcid48 = CNT_bcid48msb(rfm);
  bcid48 <<= 32;
  bcid48 |= CNT_bcid48lsb(rfm);
  bcid48 -= ctx->task[rfm].bcid48_offset;
  // copy frame
  word16 = (bcid48 >> 32) & 0xFFFF;
  word32 = (alt_u32)bcid48; // truncates
  hr_data_cursor = buf_wr_16(hr_data_cursor, word16);
  hr_data_cursor = buf_wr_32(hr_data_cursor, word32);
  bcid24 = CNT_bcid24(rfm);
  word8 = bcid24 >> 16;
  word16 = bcid24 & 0xFFFF;
  hr_data_cursor = buf_wr_8(hr_data_cursor, word8);
  hr_data_cursor = buf_wr_16(hr_data_cursor, word16);
  // unused "nb_lines"
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xFF);
  // HR DAQ chunk
  int last_hr_id = -1;
  int hr_id;
  while (!DAQ_fifo_empty(rfm)) {
    // read HR id
    word32 = p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG];
    hr_id = (int)(word32 >> 24);
    // if new HR id, insert (trailer and) header
    if (hr_id != last_hr_id) {
      if (last_hr_id >= 0) { // hr trailer
        hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xA3);
      }
      hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xB4); // hr header
    }
    hr_data_cursor = buf_wr_32(hr_data_cursor, word32);
    for (i = 0; i < 4; i++) {
      hr_data_cursor = buf_wr_32(hr_data_cursor,
                                 p_fifo_daq[rfm][ALTERA_AVALON_FIFO_DATA_REG]);
    }
    last_hr_id = hr_id;
  }
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xA3); // last hr trailer
  hr_data_cursor = buf_wr_8(hr_data_cursor, (alt_u8)0xA0); // DIF DAQ trailer
  hr_data_cursor = buf_wr_16(hr_data_cursor, (alt_u16)0xC0C0); // CRC

  ctx->task[rfm].end = hr_data_cursor;
  daq_cycle_id[rfm]++;
  return;
}
