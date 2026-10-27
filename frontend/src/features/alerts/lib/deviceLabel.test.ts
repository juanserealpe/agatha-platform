import { describe, expect, it } from "vitest";
import { deviceDisplayName, deviceOptionLabel } from "./deviceLabel";

describe("deviceDisplayName", () => {
    it.each([
        ["PK34-470", "Sensor PK 34+470"],
        ["PK34+500", "Sensor PK 34+500"],
        ["pk 2+100", "Sensor PK 02+100"],
    ])("%s → %s", (name, expected) => {
        expect(deviceDisplayName(name)).toBe(expected);
    });

    it("sin PK reconocible usa el nombre tal cual", () => {
        expect(deviceDisplayName("MP-11556")).toBe("Sensor MP-11556");
    });
});

describe("deviceOptionLabel", () => {
    const devices = [{ id: 11556, name: "PK34-470" }];

    it("usa el nombre del dispositivo de la lista", () => {
        expect(deviceOptionLabel(11556, devices)).toBe("Sensor PK 34+470");
    });

    it("con un dispositivo fuera de la lista muestra su id", () => {
        expect(deviceOptionLabel(99, devices)).toBe("Sensor 99");
    });
});
