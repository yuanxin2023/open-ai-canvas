import { create } from "zustand";

export const useModelCatalogStore = create<{
    status: "idle" | "loading" | "ready" | "error";
}>()(() => ({ status: "idle" }));
