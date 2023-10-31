#ifndef EDA_CONFIG_H
#define EDA_CONFIG_H 1

// Copyright 2021 The tomuvol-daq Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <stdio.h>

#include "alt_types.h"

int read_th_offset(FILE *dac_floor_file, alt_u32 *dac_floor_table);

int read_pa_gain(FILE *pa_gain_file, alt_u32 *pa_gain_table);

int read_mask(FILE *mask_file, alt_u32 *mask_table);

#endif // !EDA_CONFIG_H
