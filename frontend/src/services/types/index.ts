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
