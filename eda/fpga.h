#ifndef EDA_FPGA_H
#define EDA_FPGA_H 1

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

#include <stdio.h>

#ifdef TMV_ENV
#define soc_cv_av // choose cyclone V SoC
#include "hwlib.h"
#include "soc_cv_av/socal/alt_gpio.h"
#include "soc_cv_av/socal/hps.h"
#include "soc_cv_av/socal/socal.h"
#endif

//#include "altera_avalon_pio_regs.h"
#include "altera_avalon_fifo_regs.h"

#ifndef TMV_ENV
#define ALT_LWFPGASLVS_OFST 0xff200000
#else
/* defined in "soc_cv_av/socal/hps.h"

 lightweight hps2fpga bridge base address:

#define ALT_LWFPGASLVS_OFST        0xff200000

 */
#endif

#define NB_HR 8
#define NB_RFM 4
#define NB_BITS_CFG_HR 872
#define NB_BYTES_CFG_HR 109

// settings for the lightweight HPS to FPGA bridge
#define LW_H2F_BASE (ALT_LWFPGASLVS_OFST)
#define LW_H2F_SPAN (0x00100000) // enough
#define LW_H2F_MASK (LW_H2F_SPAN - 1)

// setting for the HPS to FPGA AXI Bridge
#define H2F_BASE (0xC0000000)
#define H2F_SPAN (0x00100000) // enough
#define H2F_MASK (H2F_SPAN - 1)

// slave addresses (prefixed by the bridge used)
// see ../FPGA_SoC/soc_system/soc_system.html
#define H2F_FIFO_DAQ_RFM0 0x00000098
#define H2F_FIFO_DAQ_RFM1 0x00000090
#define H2F_FIFO_DAQ_RFM2 0x00000088
#define H2F_FIFO_DAQ_RFM3 0x00000080

#define H2F_FIFO_DAQ_CSR_RFM0 0x00000040
#define H2F_FIFO_DAQ_CSR_RFM1 0x00000060
#define H2F_FIFO_DAQ_CSR_RFM2 0x00000020
#define H2F_FIFO_DAQ_CSR_RFM3 0x00000000

#define LW_H2F_LED_PIO 0x00010040
#define LW_H2F_BUTTON_PIO 0x000100C0

#define LW_H2F_RAM_SC_RFM0 0x00011000
#define LW_H2F_RAM_SC_RFM1 0x00010C00
#define LW_H2F_RAM_SC_RFM2 0x00010800
#define LW_H2F_RAM_SC_RFM3 0x00010400

#define LW_H2F_PIO_SC_CHECK_RFM0 0x00010290
#define LW_H2F_PIO_SC_CHECK_RFM1 0x00010210
#define LW_H2F_PIO_SC_CHECK_RFM2 0x000100F0
#define LW_H2F_PIO_SC_CHECK_RFM3 0x00010200

#define LW_H2F_PIO_STATE_IN 0x000100A0
#define LW_H2F_PIO_INTERRUPTS_BUS 0x00010300
#define LW_H2F_PIO_INTERRUPTS_MASK 0x00010302
#define LW_H2F_PIO_CTRL_OUT 0x00010060
#define LW_H2F_PIO_PULSER 0x00010020

#define LW_H2F_PIO_CNT_HIT0_RFM0 0x00010280
#define LW_H2F_PIO_CNT_HIT0_RFM1 0x00010260
#define LW_H2F_PIO_CNT_HIT0_RFM2 0x000100D0
#define LW_H2F_PIO_CNT_HIT0_RFM3 0x00010050

#define LW_H2F_PIO_CNT_HIT1_RFM0 0x00010270
#define LW_H2F_PIO_CNT_HIT1_RFM1 0x000100E0
#define LW_H2F_PIO_CNT_HIT1_RFM2 0x00010090
#define LW_H2F_PIO_CNT_HIT1_RFM3 0x00010010

#define LW_H2F_PIO_CNT_TRIG 0x00010250
#define LW_H2F_PIO_CNT48_MSB 0x00010240
#define LW_H2F_PIO_CNT48_LSB 0x00010230
#define LW_H2F_PIO_CNT24 0x00010220

// masks for PIO_STATE_IN
#define O_HR_TRANSMITON_0 0x00000001
#define O_CHIPSAT_0 0x00000002
#define O_HR_END_RO_0 0x00000004
#define O_SC_DONE_0 0x00000008
#define O_FIFO_IN_VALID_0 0x00000010
#define O_ALERT_0 0x00000020

#define O_HR_TRANSMITON_1 0x00000040
#define O_CHIPSAT_1 0x00000080
#define O_HR_END_RO_1 0x00000100
#define O_SC_DONE_1 0x00000200
#define O_FIFO_IN_VALID_1 0x00000400
#define O_ALERT_1 0x00000800

#define O_HR_TRANSMITON_2 0x00001000
#define O_CHIPSAT_2 0x00002000
#define O_HR_END_RO_2 0x00004000
#define O_SC_DONE_2 0x00008000
#define O_FIFO_IN_VALID_2 0x00010000
#define O_ALERT_2 0x00020000

#define O_HR_TRANSMITON_3 0x00040000
#define O_CHIPSAT_3 0x00080000
#define O_HR_END_RO_3 0x00100000
#define O_SC_DONE_3 0x00200000
#define O_FIFO_IN_VALID_3 0x00400000
#define O_ALERT_3 0x00800000

#define O_PLL_LCK 0x01000000

#define SHIFT_SYNCHRO_STATE 28

// mask for sDCC command monitoring (via PIO_CNT24)
#define SHIFT_CMD_CODE_NOW 24
#define SHIFT_CMD_CODE_MEM 28

// masks for PIO_CTRL_OUT
#define O_RESET 0x00000001
#define O_ON_OFF_RFM0 0x00000002
#define O_ON_OFF_RFM1 0x00000004
#define O_ON_OFF_RFM2 0x00000008
#define O_ON_OFF_RFM3 0x00000010
#define O_ENA_RFM0 0x00000020
#define O_ENA_RFM1 0x00000040
#define O_ENA_RFM2 0x00000080
#define O_ENA_RFM3 0x00000100
#define O_RESET_HR 0x00000200
#define O_SELECT_SC_RR 0x00000400
#define O_RESET_SC 0x00000800
#define O_START_SC_0 0x00001000
#define O_START_SC_1 0x00002000
#define O_START_SC_2 0x00004000
#define O_START_SC_3 0x00008000
#define O_SEL_CMD_SOURCE 0x00010000
#define SHIFT_CMD_CODE 17
#define O_RST_SCALERS 0x00200000
#define O_ENA_SCALERS 0x00400000
#define O_SEL_TRIG_THRESH 0x00800000
#define O_ENA_TRIG 0x01000000
#define O_CTEST 0x02000000
#define O_HPS_BUSY 0x04000000
#define O_ENA_DCC_BUSY 0x08000000
#define O_ENA_DCC_RAMFULL 0x10000000
#define O_BBL_RST 0x20000000

#define EDA_DIF_ID_OFFS 0x00

// synchro states
#define S_IDLE 0
#define S_RST_CNT48 1
#define S_RST_CNT24 2
#define S_ACQ 3
#define S_RAMFULL 4
#define S_START_RO 5
#define S_WAIT_END_RO 6
#define S_FIFO_READY 7
#define S_STOP_RUN 8

// interrupts
#define PIO_IRQ 3
#define IRQ_RST_BCID 1
#define IRQ_ACQ 2
#define IRQ_RO 4
#define IRQ_FIFO_READY 8

// commands
#define CMD_RESET_BCID 1
#define CMD_START_ACQ 2
#define CMD_RAMFULL_EXT 3
#define CMD_STOP_ACQ 5
#define CMD_DIGITAL_RO 6
#define CMD_IDLE 0xE

// memory-mapping
int mmap_lw_h2f(int fd);
int mmap_h2f(int fd);
int munmap_lw_h2f(int fd);
int munmap_h2f(int fd);

// pio general
void PIO_set_defaults();
alt_u32 PIO_state_get();
alt_u32 PIO_ctrl_get();

// i2c general
int I2C_init();
void I2C_close();

// RFM on-off, enable
void RFM_on(int rfm);
void RFM_off(int rfm);
void RFM_enable(int rfm);
void RFM_disable(int rfm);

// monitoring
int MONIT_pwr_alert(int rfm);
int MONIT_PTH_select();
int MONIT_PTH_read_id();
int MONIT_PT100_select();
int MONIT_PT100_write_reg(alt_u8 reg_addr, alt_u8 value);
int MONIT_PWRMON_select(int rfm);
int MONIT_PWRMON_readreg(alt_u8 reg_addr);
void MONIT_BBL_reset();

// clocks and synchronization
void SYNC_reset_fpga();
void SYNC_reset_hr();
int SYNC_pll_lck();
int SYNC_state();
void SYNC_select_command_soft();
void SYNC_select_command_dcc();
void SYNC_set_command(alt_u32 cmd);
void SYNC_reset_bcid();
void SYNC_start_acq();
void SYNC_stop_acq();
void SYNC_ramfull_ext();
void SYNC_digital_ro();
void SYNC_fifo_arming();
void SYNC_fifo_ack();
int SYNC_dcc_cmd_mem();
int SYNC_dcc_cmd_now();
int SYNC_ramfull();
int SYNC_fpga_ro();
int SYNC_fifo_ready();
int SYNC_run_stopped();
int SYNC_hr_transmiton(int rfm);
int SYNC_chipsat(int rfm);
int SYNC_hr_end_ro(int rfm);
void SYNC_enable_dcc_busy();
void SYNC_enable_dcc_ramfull();

// trigger ("hit" level) configuration
void TRIG_select_thresh_0();
void TRIG_select_thresh_1();
void TRIG_enable();
void TRIG_disable();

// counters (time, events)
void CNT_reset();
void CNT_start();
void CNT_stop();
alt_u32 CNT_hit0(int rfm);
alt_u32 CNT_hit1(int rfm);
alt_u32 CNT_trig();
alt_u32 CNT_bcid24();
alt_u32 CNT_bcid48msb();
alt_u32 CNT_bcid48lsb();
void CNT_save(FILE *output, int rfm);

// Hardroc slow control registers
void HRSC_select_slow_control();
void HRSC_select_read_register();
void HRSC_set_bit(alt_u32 hr_addr, alt_u32 bit_addr, alt_u32 bit);
alt_u32 HRSC_get_bit(alt_u32 hr_addr, alt_u32 bit_addr);
void HRSC_set_word(alt_u32 hr_addr, alt_u32 bit_addr, alt_u32 n_bits,
                   alt_u32 value);
void HRSC_set_word_msb2lsb(alt_u32 hr_addr, alt_u32 bit_addr, alt_u32 n_bits,
                           alt_u32 value);
int HRSC_read_conf_singl(FILE *file_conf, alt_u32 hr_addr);
void HRSC_copy_conf(alt_u32 hr_addr_source, alt_u32 hr_addr_dest);
int HRSC_read_conf_mult(FILE *file_conf);
int HRSC_write_conf_mult(FILE *file_conf);
void HRSC_set_ctest(alt_u32 hr_addr, alt_u32 chan, alt_u32 val);
void HRSC_set_all_ctest_off();
void HRSC_set_preamp(alt_u32 hr_addr, alt_u32 chan, alt_u32 val);
void HRSC_set_shaper_capa(alt_u32 hr_addr, alt_u32 val);
void HRSC_set_shaper_resis(alt_u32 hr_addr, alt_u32 val);
void HRSC_set_cmd_fsb2(alt_u32 hr_addr, alt_u32 chan, alt_u32 val);
void HRSC_set_cmd_fsb1(alt_u32 hr_addr, alt_u32 chan, alt_u32 val);
void HRSC_set_mask(alt_u32 hr_addr, alt_u32 chan, alt_u32 val);
void HRSC_set_chip_id(alt_u32 hr_addr, alt_u32 val);
void HRSC_set_DAC0(alt_u32 hr_addr, alt_u32 val);
void HRSC_set_DAC1(alt_u32 hr_addr, alt_u32 val);
void HRSC_set_DAC2(alt_u32 hr_addr, alt_u32 val);
void HRSC_set_DAC_coarse(alt_u32 hr_addr);
void HRSC_set_DAC_fine(alt_u32 hr_addr);
int HRSC_set_config(int rfm);
int HRSC_reset_read_registers(int rfm);
int HRSC_set_read_register(int rfm, int channel);
void HRSC_reset_sc();
void HRSC_start_sc(int rfm);
int HRSC_sc_done(int rfm);

// Hardroc DAQ
alt_u32 bit(alt_u32 word, alt_u32 digit);
void DAQ_fifo_init(int rfm);
void DAQ_fifo_print_status(int rfm);
void DAQ_fifo_print_event(int rfm);
void DAQ_fifo_clear_event(int rfm);
int DAQ_fifo_in_valid(int rfm);
int DAQ_fifo_full(int rfm);
int DAQ_fifo_empty(int rfm);
alt_u32 DAQ_fifo_fill_level(int rfm);
alt_u32 DAQ_fifo_data(int rfm);
alt_u32 DAQ_fifo_clear(int rfm);
alt_u32 DAQ_hexdump_hr_data(FILE *output, int rfm);
alt_u32 DAQ_save_hr_data(FILE *output, int rfm);
alt_u32 DAQ_save_hr_data_DIF(FILE *output, int rfm);
void DAQ_bufferize_data_DIF(int rfm);
int DAQ_send_sock_buffer(int sock);
int DAQ_write_buffer(FILE *f);
void DAQ_reset_buffer();

// Test injector
int INJ_dac_select(int rfm);
int INJ_dac_set(alt_u32 value);
void INJ_start_ctest_pulser(alt_u32 half_period, alt_u32 phase);
// half period unit = 200 ns
// phase unit = 6.25 ns
void INJ_stop_ctest_pulser();

// Printing
void PRINT_fifo_status(int rfm);
void PRINT_counters(FILE *output, int rfm);
void PRINT_hr_data(FILE *output, alt_u32 nb_evt);
void PRINT_config(FILE *file_conf, int rfm);
void PRINT_dump_registers(FILE *output);

#endif /* !EDA_FPGA_H */
