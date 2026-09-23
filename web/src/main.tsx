import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import { bootstrapAppearance } from "@/services/appearance-bootstrap";
import { isIsolatedDirectorRepro } from "@/lib/dev-repro";

// The backend-free DEV lab must not make requests before AppProviders isolates it.
const appearanceReady = isIsolatedDirectorRepro(import.meta.env.DEV, window.location.pathname) ? Promise.resolve() : bootstrapAppearance();
void appearanceReady.finally(() => import("./application"));
