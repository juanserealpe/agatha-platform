import { describe, expect, it } from "vitest";
import type { Device } from "@/services/dashboard/dashboard.types";
import { filterDevices, matchesSensorQuery } from "./sensorSearch";

const device: Device = { id: 1, name: "Sensor MP-11556 PK34-470", lastConnection: null, lastBatteryLevel: null };

describe("matchesSensorQuery", () => {
    it.each(["", "  ", "sensor", "SÉNSOR", "11556", "mp-115"])("coincide por nombre: %j", (query) => {
        expect(matchesSensorQuery(device, query)).toBe(true);
    });

    it.each(["34+470", "34-470", "pk 34 + 470", "34470", "34", "34+47", "3", "344", "3447", "pk344"])("coincide por PK: %j", (query) => {
        expect(matchesSensorQuery(device, query)).toBe(true);
    });

    it.each(["35", "345", "34471", "34+471", "38+174", "xyz"])("no coincide: %j", (query) => {
        expect(matchesSensorQuery(device, query)).toBe(false);
    });
});

describe("filterDevices", () => {
    it("devuelve solo los dispositivos que coinciden", () => {
        const other: Device = { ...device, id: 2, name: "Sensor MP-11557 PK38-174" };
        expect(filterDevices([device, other], "38+1")).toEqual([other]);
    });
});
