#ifndef EDA_LOGGER_H
#define EDA_LOGGER_H 1

// Copyright 2021 The tomuvol-daq Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <stdio.h>

void log_init();
void log_flush();
void log_printf(const char *fmt, ...);
void log_close();

#endif // !EDA_LOGGER_H
