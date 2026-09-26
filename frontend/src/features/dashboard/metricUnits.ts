export const METRIC_UNIT_OVERRIDES: Record<number, string> = {
    1: "°", 2: "°", 15: "°", 16: "°", 19: "°",     // orientación y deriva
    8: "g",                                          // magnitud promedio
    9: "g/s",                                        // actividad pico
    6: "cuentas ADC",                                // RMS (ADC)
    17: "cuentas ADC",                               // RMS acústico
    11: "Hz", 12: "Hz", 13: "Hz", 14: "Hz",              // bandas B0-B3, por mil
    10: "",                                          // razón R, adimensional
    18: "",                                          // planitud espectral (0.0625 a 1, sin unidad física)
    20: "°C",
    21: "% HR",
    5: "%",
    3: "mV",
    4: "mA",
    7: "%",                                          // Actividad 
};