import { afterAll, afterEach, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
import type { PaymentOrder } from "../../src/services/api/payments";

const dom = new Window({ url: "http://localhost/canvas/test" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, Event: dom.Event, CustomEvent: dom.CustomEvent, IS_REACT_ACT_ENVIRONMENT: true })) {
    Object.defineProperty(globalThis, key, { configurable: true, value });
}
const notices: string[] = [];
const message = Object.fromEntries(["success", "error", "info"].map((key) => [key, (text: string) => notices.push(text)]));
mock.module("antd", () => ({ App: { useApp: () => ({ message }) } }));
type Result = { order: PaymentOrder };
let requests: Array<{ productId: string; providerId: string; idempotencyKey: string }> = [];
let create: () => Promise<Result>;
let read: () => Promise<Result>;
let query: () => Promise<Result>;
let cancel: () => Promise<Result>;
let retry: () => Promise<Result>;
let redeemCode: (code: string) => Promise<unknown>;
mock.module("@/services/api/payments", () => ({
    createPaymentOrder: (input: (typeof requests)[number]) => {
        requests.push(input);
        return create();
    },
    getPaymentOrder: () => read(),
    queryPaymentOrder: () => query(),
    closePaymentOrder: () => cancel(),
    refreshPaymentCheckout: () => retry(),
}));
mock.module("@/services/api/wallet", () => ({ redeemCredits: (code: string) => redeemCode(code) }));
const { act, useEffect, StrictMode } = await import("react");
const { createRoot } = await import("react-dom/client");
const { usePaymentOrderFlow } = await import("../../src/components/payment/use-payment-order-flow");
const { useCreditRedemption } = await import("../../src/hooks/use-credit-redemption");
const { useUserStore } = await import("../../src/stores/use-user-store");
const initialStore = useUserStore.getState();
const user = { id: "user-a" } as NonNullable<typeof initialStore.user>;
let grants = 0;
let updates = 0;
let flow: ReturnType<typeof usePaymentOrderFlow>;
let redemption: ReturnType<typeof useCreditRedemption>;
let root: ReturnType<typeof createRoot> | undefined;
let host: HTMLDivElement | undefined;
const polls = new Map<number, () => void>();
const originalInterval = window.setInterval.bind(window);
window.setInterval = ((callback: () => void, delay: number) => {
    const id = originalInterval(callback, delay);
    if (delay === 2000) polls.set(id, callback);
    return id;
}) as typeof window.setInterval;
window.addEventListener("wallet:updated", () => updates++);

function order(status: PaymentOrder["status"] = "pending"): PaymentOrder {
    return {
        id: "order-a",
        productId: "removed-product",
        providerId: "removed-provider",
        status,
        currency: "CNY",
        amountFen: 990,
        creditsMicrocredits: 100000000,
        checkout: { mode: "qr_code", value: "weixin://test" },
        expiresAt: new Date(Date.now() + 60000).toISOString(),
    } as PaymentOrder;
}
function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((done) => {
        resolve = done;
    });
    return { promise, resolve };
}
async function mount(strictReturn = false) {
    requests = [];
    notices.length = 0;
    grants = 0;
    updates = 0;
    polls.clear();
    useUserStore.setState({ user, features: { ...initialStore.features, creditsEnabled: true } });
    create = read = query = cancel = retry = async () => ({ order: order() });
    redeemCode = async () => ({});
    host = document.createElement("div");
    document.body.append(host);
    root = createRoot(host);
    function Harness() {
        flow = usePaymentOrderFlow({
            onGranted: () => {
                grants++;
            },
        });
        redemption = useCreditRedemption(user.id);
        useEffect(() => {
            if (strictReturn) void flow.showOrder("order-a");
        }, []);
        return null;
    }
    await act(async () =>
        root!.render(
            strictReturn ? (
                <StrictMode>
                    <Harness />
                </StrictMode>
            ) : (
                <Harness />
            ),
        ),
    );
}
afterEach(async () => {
    if (root) await act(async () => root!.unmount());
    root = undefined;
    host?.remove();
    useUserStore.setState(initialStore);
});
afterAll(() => dom.happyDOM.abort());

test("development effect replay can reload a returning payment order", async () => {
    await mount(true);
    expect(flow.order?.id).toBe("order-a");
    expect(flow.querying).toBe(false);
});

test("duplicate clicks create one order; ambiguous failure reuses its idempotency key", async () => {
    await mount();
    const pending = deferred<Result>();
    create = () => pending.promise;
    let started!: Promise<void>;
    await act(async () => {
        started = flow.start("product", "provider");
        await flow.start("product", "provider");
    });
    expect(requests).toHaveLength(1);
    await act(async () => {
        pending.resolve({ order: order() });
        await started;
    });
    expect(flow.order?.id).toBe("order-a");
    await act(async () => flow.close());
    create = async () => {
        throw new Error("network lost");
    };
    await act(async () => flow.start("product", "provider"));
    const key = requests.at(-1)!.idempotencyKey;
    create = async () => ({ order: order() });
    await act(async () => flow.start("product", "provider"));
    expect(requests.at(-1)!.idempotencyKey).toBe(key);
    await act(async () => {
        flow.close();
        flow.resetIdempotency();
    });
    await act(async () => flow.start("other-product", "provider"));
    expect(requests.at(-1)!.idempotencyKey).not.toBe(key);
});

test("payment returns use order snapshots even when a product is removed; repeated credited results grant once", async () => {
    await mount();
    read = query = async () => ({ order: order("credited") });
    await act(async () => flow.showOrder("order-a"));
    expect(flow.order?.productId).toBe("removed-product");
    await act(async () => flow.refresh());
    await act(async () => flow.showOrder("order-a"));
    expect(grants).toBe(1);
    expect(notices.filter((text) => text.includes("支付成功"))).toHaveLength(1);
});

test("pending order polling settles payment and stops when checkout closes", async () => {
    await mount();
    await act(async () => flow.start("product", "provider"));
    read = async () => ({ order: order("credited") });
    const poll = [...polls.values()].at(-1)!;
    await act(async () => {
        poll();
        await Promise.resolve();
    });
    expect(flow.order?.status).toBe("credited");
    expect(grants).toBe(1);
    await act(async () => flow.close());
    expect(flow.order).toBeNull();
});

test("close reports late-payment uncertainty; checkout retry adopts the server order", async () => {
    await mount();
    await act(async () => flow.start("product", "provider"));
    cancel = async () => ({ order: { ...order("closed"), closurePending: true } });
    await act(async () => flow.cancel());
    expect(flow.order?.closurePending).toBe(true);
    expect(notices.at(-1)).toContain("若已支付");
    read = async () => ({ order: order("create_failed") });
    await act(async () => flow.showOrder("order-a"));
    await act(async () => flow.retry());
    expect(flow.order?.status).toBe("pending");
});

test("responses after account switches and unmounts cannot update checkout or grant notifications", async () => {
    await mount();
    const pending = deferred<Result>();
    create = () => pending.promise;
    let started!: Promise<void>;
    await act(async () => {
        started = flow.start("product", "provider");
    });
    await act(async () => {
        useUserStore.setState({ user: { ...user, id: "user-b" } });
    });
    await act(async () => {
        pending.resolve({ order: order("credited") });
        await started;
    });
    expect(flow.order).toBeNull();
    expect(grants).toBe(0);
    expect(notices).toHaveLength(0);
    await act(async () => {
        useUserStore.setState({ user });
    });
    const returning = deferred<Result>();
    read = () => returning.promise;
    let loading!: Promise<void>;
    await act(async () => {
        loading = flow.showOrder("order-a");
    });
    await act(async () => root!.unmount());
    root = undefined;
    await act(async () => {
        returning.resolve({ order: order("credited") });
        await loading;
    });
    expect(grants).toBe(0);
});

test("closing checkout invalidates a delayed return response", async () => {
    await mount();
    const pending = deferred<Result>();
    read = () => pending.promise;
    let loading!: Promise<void>;
    await act(async () => {
        loading = flow.showOrder("order-a");
    });
    await act(async () => flow.close());
    await act(async () => {
        pending.resolve({ order: order("credited") });
        await loading;
    });
    expect(flow.order).toBeNull();
    expect(grants).toBe(0);
});

test("redemption normalizes the code, prevents duplicate writes, refreshes balance only on success and preserves failed input", async () => {
    await mount();
    await act(async () => redemption.setCode("A".repeat(32)));
    const pending = deferred<unknown>();
    const codes: string[] = [];
    redeemCode = (code) => {
        codes.push(code);
        return pending.promise;
    };
    let started!: Promise<void>;
    await act(async () => {
        started = redemption.redeem();
        await redemption.redeem();
    });
    expect(codes).toEqual(["a".repeat(32)]);
    await act(async () => {
        pending.resolve({});
        await started;
    });
    expect(updates).toBe(1);
    expect(redemption.code).toBe("");
    await act(async () => redemption.setCode("b".repeat(32)));
    redeemCode = async () => {
        throw new Error("invalid code");
    };
    await act(async () => redemption.redeem());
    expect(redemption.code).toBe("b".repeat(32));
    expect(updates).toBe(1);
    expect(notices.at(-1)).toBe("invalid code");
});

test("redemption after an account switch does not announce or refresh the next account", async () => {
    await mount();
    await act(async () => redemption.setCode("a".repeat(32)));
    const pending = deferred<unknown>();
    redeemCode = () => pending.promise;
    let started!: Promise<void>;
    await act(async () => {
        started = redemption.redeem();
    });
    await act(async () => {
        useUserStore.setState({ user: { ...user, id: "user-b" } });
    });
    await act(async () => {
        pending.resolve({});
        await started;
    });
    expect(updates).toBe(0);
    expect(notices).toHaveLength(0);
});
