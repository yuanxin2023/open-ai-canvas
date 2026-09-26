import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import { bootstrapAppearance } from "@/services/appearance-bootstrap";
import { isIsolatedDirectorRepro } from "@/lib/dev-repro";
import { migrateFirstPartyStorageKeys } from "@/lib/first-party-storage-migration";

// The backend-free DEV lab must not make requests before AppProviders isolates it.
migrateFirstPartyStorageKeys();
const appearanceReady = isIsolatedDirectorRepro(import.meta.env.DEV, window.location.pathname) ? Promise.resolve() : bootstrapAppearance();
void appearanceReady.finally(() => import("./application"));
