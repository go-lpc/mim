#ifndef EDA_DEVICE_H
#define EDA_DEVICE_H 1

/*
 * Copyright 2020 Baptiste Joly <baptiste.joly@clermont.in2p3.fr>
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

#include "fpga.h"
#include <stdint.h>

typedef struct Device {
  // run-dependant settings
  alt_u32 thresh_delta;
  alt_u32 rshaper;
  alt_u32 rfm_on;

  char *ip_addr;
  int run_cnt;

  int trig_mode; // 0: dcc, 1:soft
  // baseline settings
  char cfg_mode[4]; // 'db' or 'csv'
  alt_u32 dac_floor_table[NB_RFM * NB_HR * 3];
  alt_u32 pa_gain_table[NB_RFM * NB_HR * 64];
  alt_u32 mask_table[NB_RFM * NB_HR * 64];

  int mem_fd;

  char daq_filename[128];
  FILE *daq_file;
  alt_u32 cycle_id; // run counters

  struct {
    uint8_t dif;
    uint32_t rshaper;
    char *addr;
    int port;
    int sck;

    uint8_t *beg;
    uint8_t *end;
    alt_u32 bcid48_offset;

    int rc;
  } task[NB_RFM]; // run-loop status
} Device_t;

Device_t *new_device();

int device_boot_rfm(Device_t *ctx, uint8_t dif, int slot, uint32_t rshaper,
                    uint32_t trig);
int device_configure_dif(Device_t *ctx, uint8_t dif, const char *addr,
                         int port);
int device_configure(Device_t *ctx, uint32_t thresh, uint32_t rshaper,
                     uint32_t rfm, const char *ip, int run);
int device_initialize(Device_t *ctx);
int device_start(Device_t *ctx, uint32_t run);
void device_loop(Device_t *ctx);
int device_stop(Device_t *ctx);
void device_stop_loop(Device_t *ctx);

void device_free(Device_t *ctx);

int device_init_mmap(Device_t *ctx);
int device_init_fpga(Device_t *ctx);
int device_init_hrsc(Device_t *ctx);
int device_init_scks(Device_t *ctx);

#endif // !EDA_DEVICE_H
