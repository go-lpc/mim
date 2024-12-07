// Copyright 2021 The tomuvol-daq Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "logger.h"

#include <stdarg.h>
#include <string.h>

// fixed-location, temporary, line-buffered log file
FILE *logfile;
char logbuf[128];

void log_init() {
  logfile = fopen("/dev/shm/log.out", "w");
  memset(logbuf, '\0', 128);
  setvbuf(logfile, logbuf, _IOLBF, 128);
  return;
}

void log_printf(const char *fmt, ...) {
  va_list arglist;
  va_start(arglist, fmt);
  vsprintf(logbuf, fmt, arglist);
  va_end(arglist);
}

void log_flush() {
  fprintf(stderr, "%s", logbuf);
  fprintf(logfile, "%s", logbuf);
  memset(logbuf, '\0', 128);
  return;
}

void log_close() {
  log_flush();
  fclose(logfile);
  return;
}
