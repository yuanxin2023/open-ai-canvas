import { ApiError } from "@/services/api/request";

export const IMAGE_CREDITS_SHORTFALL_EVENT = "image:credits-shortfall";
export const INSUFFICIENT_CREDITS_REASON = "insufficient_credits";

export function isInsufficientCreditsError(error: unknown): error is ApiError {
    return error instanceof ApiError && error.reason === INSUFFICIENT_CREDITS_REASON;
}

export function notifyImageCreditsShortfall(error: unknown) {
    if (!isInsufficientCreditsError(error) || typeof window === "undefined") return false;
    window.dispatchEvent(new Event(IMAGE_CREDITS_SHORTFALL_EVENT));
    return true;
}
