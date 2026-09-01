export interface FilterOption {
    value: string;
    label: string;
}

export interface FiltersState {
    gasoducto: string;
    sensor: string;
    criticidad: string;
}

export type NivelAlerta = "verde" | "amarilla" | "roja";

export type TipoAlerta = "movimiento_terreno" | "fuga";

export type TipoTuberia = "flexible" | "metalico";

export interface Sensor {
    id: string;
    pk: string;
    nombre: string;
    nivelAlerta: NivelAlerta;
    tipoAlerta?: TipoAlerta;
    bateria: number;
    ultimaComunicacion: string;
    senalCelular: number;
    coordenadas: { lat: number; lng: number };
    descripcionEvento?: string; // ej. "El tubo se movió 5 cm de su posición original"
    confianzaDeteccion?: number; // 0-100
    aceleracion?: "baja" | "media" | "alta";
    esDatoReal?: boolean;
    tipoTuberia?: TipoTuberia;
}

export type MotivoObservacion =
    | "fuga_reportada"
    | "falla_sensor"
    | "actividad_humana"
    | "condiciones_climaticas";

export interface AlertaExport {
    nivelAlerta: NivelAlerta;
    tipo: TipoAlerta;
    sensorId: string;
    pk: string;
    coordenadas: { lat: number; lng: number };
    observaciones: string | null;
    lecturasSensor: Record<string, number>; // ej. { temperatura: 24.5, humedad: 60 }
    umbralesAlerta: Record<string, number>; // el umbral que se cruzó, para explicar el porqué
}

export const TIPOS_ALERTA: Record<TipoAlerta, string> = {
    movimiento_terreno: "Riesgo Movimiento de Terreno",
    fuga: "Riesgo Fuga",
};

export const MOTIVOS_OBSERVACION: Record<MotivoObservacion, string> = {
    fuga_reportada: "Fuga reportada",
    falla_sensor: "Falla de sensor",
    actividad_humana: "Actividad humana",
    condiciones_climaticas: "Condiciones climáticas",
};