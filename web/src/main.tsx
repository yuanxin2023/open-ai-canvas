import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import { bootstrapAppearance } from "@/services/appearance-bootstrap";

void bootstrapAppearance().finally(() => import("./application"));
