import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { memoryStorage } from "./testing";

const STORAGE_KEY = "agatha.session";

const valid = { accessToken: "tok-1", expiresAt: "2099-01-01T00:00:00Z", username: "admin" };

async function loadStore() {
    // Módulo nuevo en cada prueba: los listeners y el listener de `storage` son globales.
    vi.resetModules();
    return (await import("./tokenStore")).tokenStore;
}

let storage: Storage;

beforeEach(() => {
    storage = memoryStorage();
    vi.stubGlobal("localStorage", storage);
});

afterEach(() => {
    vi.unstubAllGlobals();
});

describe("tokenStore", () => {
    it("guarda, lee y borra la sesión", async () => {
        const tokenStore = await loadStore();
        expect(tokenStore.get()).toBeNull();

        tokenStore.set(valid);
        expect(JSON.parse(storage.getItem(STORAGE_KEY)!)).toEqual(valid);
        expect(tokenStore.get()).toEqual(valid);
        expect(tokenStore.accessToken()).toBe("tok-1");

        tokenStore.clear();
        expect(storage.getItem(STORAGE_KEY)).toBeNull();
        expect(tokenStore.accessToken()).toBeNull();
    });

    it("descarta una sesión vencida sin esperar al 401", async () => {
        storage.setItem(STORAGE_KEY, JSON.stringify({ ...valid, expiresAt: "2000-01-01T00:00:00Z" }));
        const tokenStore = await loadStore();

        expect(tokenStore.get()).toBeNull();
        expect(storage.getItem(STORAGE_KEY)).toBeNull();
    });

    it("descarta un valor guardado que no es JSON", async () => {
        storage.setItem(STORAGE_KEY, "{roto");
        const tokenStore = await loadStore();

        expect(tokenStore.get()).toBeNull();
        expect(storage.getItem(STORAGE_KEY)).toBeNull();
    });

    it("avisa a los suscriptores al guardar y al borrar, hasta que se desuscriben", async () => {
        const tokenStore = await loadStore();
        const listener = vi.fn();
        const unsubscribe = tokenStore.subscribe(listener);

        tokenStore.set(valid);
        tokenStore.clear();
        unsubscribe();
        tokenStore.set(valid);

        expect(listener.mock.calls).toEqual([[valid], [null]]);
    });

    it("propaga el cambio hecho en otra pestaña", async () => {
        const win = new EventTarget();
        vi.stubGlobal("window", win);
        const tokenStore = await loadStore();
        const listener = vi.fn();
        tokenStore.subscribe(listener);

        // Otra pestaña inició sesión.
        storage.setItem(STORAGE_KEY, JSON.stringify(valid));
        win.dispatchEvent(Object.assign(new Event("storage"), { key: STORAGE_KEY }));
        // Una clave ajena no cuenta.
        win.dispatchEvent(Object.assign(new Event("storage"), { key: "otra" }));

        expect(listener.mock.calls).toEqual([[valid]]);
    });

    it("sin localStorage disponible funciona como si no hubiera sesión", async () => {
        vi.stubGlobal("localStorage", undefined);
        const tokenStore = await loadStore();

        expect(tokenStore.get()).toBeNull();
        expect(() => tokenStore.set(valid)).not.toThrow();
        expect(() => tokenStore.clear()).not.toThrow();
    });
});
