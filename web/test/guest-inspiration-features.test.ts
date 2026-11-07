import { expect, test } from "bun:test";
import type { AuthSessionPayload } from "../src/services/api/auth";
import { defaultFeatureAvailability, useUserStore } from "../src/stores/use-user-store";

test.each([true, false])("guest session applies inspiration visibility %s without retaining account features", (enabled) => {
    const previous = useUserStore.getState();
    try {
        useUserStore.getState().setFeatures({ ...defaultFeatureAvailability, frontendModelsEnabled: true, ecommerceEnabled: true });
        const payload: AuthSessionPayload = {
            user: null,
            features: { inspirationHallEnabled: enabled, inspirationSourcesVisible: false },
        };
        useUserStore.getState().setFeatures(payload.features);
        expect(useUserStore.getState().features).toEqual({ ...defaultFeatureAvailability, inspirationHallEnabled: enabled, inspirationSourcesVisible: false });
    } finally {
        useUserStore.setState(previous);
    }
});
