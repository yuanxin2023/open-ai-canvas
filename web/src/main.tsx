import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import { installChunkRecovery } from "@/lib/chunk-recovery";
import { bootstrapAppearance } from "@/services/appearance-bootstrap";
import { isIsolatedPrevisRepro } from "@/lib/dev-repro";
import { migrateFirstPartyStorageKeys } from "@/lib/first-party-storage-migration";

migrateFirstPartyStorageKeys();
installChunkRecovery();

// The backend-free DEV lab must not make appearance requests.
const appearanceReady = isIsolatedPrevisRepro(import.meta.env.DEV, window.location.pathname) ? Promise.resolve() : bootstrapAppearance();
void import("./application");
void appearanceReady.catch(() => undefined);
