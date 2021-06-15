// Copyright 2021 The tomuvol-daq Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <stdlib.h>

#include "config.h"
#include "device.h"
#include "logger.h"

int read_th_offset(FILE *dac_floor_file, alt_u32 *dac_floor_table) {
  ssize_t n = 0;
  int rfm_index, hr_addr;
  int dac_floor;
  int delimiter = ';';
  char *str = NULL;
  size_t len = 0;
  for (rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
    for (hr_addr = 0; hr_addr < NB_HR; hr_addr++) {
      n = getdelim(&str, &len, delimiter, dac_floor_file);
      if (n == 0) {
        continue;
      }
      while (str[0] == '#') { // if comment line, skip
        n = getline(&str, &len, dac_floor_file);
        if (n == 0) {
          continue;
        }
        n = getdelim(&str, &len, delimiter, dac_floor_file);
        if (n == 0) {
          continue;
        }
      }
      if (atoi(str) != rfm_index) {
        log_printf("error parsing dac_floor file\n");
        log_flush();
        return -1;
      }
      n = getdelim(&str, &len, delimiter, dac_floor_file);
      if (n == 0) {
        continue;
      }
      if (atoi(str) != hr_addr) {
        log_printf("error parsing dac_floor file\n");
        return -1;
      }
      // dac0
      n = getdelim(&str, &len, delimiter, dac_floor_file);
      if (n == 0) {
        continue;
      }
      dac_floor = atoi(str);
      dac_floor_table[3 * (NB_HR * rfm_index + hr_addr)] = dac_floor;
      // dac1
      n = getdelim(&str, &len, delimiter, dac_floor_file);
      if (n == 0) {
        continue;
      }
      dac_floor = atoi(str);
      dac_floor_table[3 * (NB_HR * rfm_index + hr_addr) + 1] = dac_floor;
      // dac2
      n = getline(&str, &len, dac_floor_file);
      if (n == 0) {
        continue;
      }
      dac_floor = atoi(str);
      dac_floor_table[3 * (NB_HR * rfm_index + hr_addr) + 2] = dac_floor;
    }
  }
  return 1;
}

int read_pa_gain(FILE *pa_gain_file, alt_u32 *pa_gain_table) {
  ssize_t n = 0;
  int rfm_index, hr_addr, chan;
  int pa_gain;
  int delimiter = ';';
  char *str = NULL;
  size_t len = 0;
  for (rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
    for (hr_addr = 0; hr_addr < NB_HR; hr_addr++) {
      for (chan = 0; chan < 64; chan++) {
        n = getdelim(&str, &len, delimiter, pa_gain_file);
        if (n == 0) {
          continue;
        }
        while (str[0] == '#') { // if comment line, skip
          n = getline(&str, &len, pa_gain_file);
          if (n == 0) {
            continue;
          }
          n = getdelim(&str, &len, delimiter, pa_gain_file);
          if (n == 0) {
            continue;
          }
        }
        if (atoi(str) != rfm_index) {
          log_printf("error parsing dac_floor file\n");
          log_flush();
          return -1;
        }
        n = getdelim(&str, &len, delimiter, pa_gain_file);
        if (n == 0) {
          continue;
        }
        if (atoi(str) != hr_addr) {
          log_printf("error parsing pa_gain file\n");
          log_flush();
          return -1;
        }
        n = getdelim(&str, &len, delimiter, pa_gain_file);
        if (n == 0) {
          continue;
        }
        if (atoi(str) != chan) {
          log_printf("error parsing pa_gain file\n");
          log_flush();
          return -1;
        }
        n = getline(&str, &len, pa_gain_file);
        if (n == 0) {
          continue;
        }
        pa_gain = atoi(str);
        pa_gain_table[64 * (NB_HR * rfm_index + hr_addr) + chan] = pa_gain;
      }
    }
  }
  return 1;
}

int read_mask(FILE *mask_file, alt_u32 *mask_table) {
  ssize_t n = 0;
  int rfm_index, hr_addr, chan;
  int mask;
  int delimiter = ';';
  char *str = NULL;
  size_t len = 0;
  for (rfm_index = 0; rfm_index < NB_RFM; rfm_index++) {
    for (hr_addr = 0; hr_addr < NB_HR; hr_addr++) {
      for (chan = 0; chan < 64; chan++) {
        n = getdelim(&str, &len, delimiter, mask_file);
        if (n == 0) {
          continue;
        }
        while (str[0] == '#') { // if comment line, skip
          n = getline(&str, &len, mask_file);
          if (n == 0) {
            continue;
          }
          n = getdelim(&str, &len, delimiter, mask_file);
          if (n == 0) {
            continue;
          }
        }
        if (atoi(str) != rfm_index) {
          log_printf("error parsing dac_floor file\n");
          log_flush();
          return -1;
        }
        n = getdelim(&str, &len, delimiter, mask_file);
        if (n == 0) {
          continue;
        }
        if (atoi(str) != hr_addr) {
          log_printf("error parsing pa_gain file\n");
          log_flush();
          return -1;
        }
        n = getdelim(&str, &len, delimiter, mask_file);
        if (n == 0) {
          continue;
        }
        if (atoi(str) != chan) {
          log_printf("error parsing pa_gain file\n");
          log_flush();
          return -1;
        }
        n = getline(&str, &len, mask_file);
        if (n == 0) {
          continue;
        }
        mask = atoi(str);
        mask_table[64 * (NB_HR * rfm_index + hr_addr) + chan] = mask;
      }
    }
  }
  return 1;
}
