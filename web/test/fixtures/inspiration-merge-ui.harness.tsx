import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
const dom = new Window({ url: "http://localhost/" });
for (const [key, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    HTMLElement: dom.HTMLElement,
    Element: dom.Element,
    Node: dom.Node,
    Event: dom.Event,
    CustomEvent: dom.CustomEvent,
    IS_REACT_ACT_ENVIRONMENT: true,
}))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { useUserStore, defaultFeatureAvailability } = await import("@/stores/use-user-store");
let modal: any, form: any;
const notices: string[] = [];
const Box = ({ children }: any) => <div>{children}</div>;
const Form = Object.assign(Box, {
    Item: Box,
    useForm() {
        const ref = React.useRef<any>(null);
        if (!ref.current)
            ref.current = {
                values: {},
                async validateFields() {
                    return { ...this.values };
                },
                setFieldsValue(values: any) {
                    this.values = { ...values };
                },
                resetFields() {
                    this.values = {};
                },
            };
        form = ref.current;
        return [form];
    },
});
const Input = Object.assign(() => null, { TextArea: () => null });
mock.module("antd", () => ({
    App: { useApp: () => ({ message: { error: (value: string) => notices.push(value) } }) },
    Form,
    Input,
    Select: () => null,
    Button: ({ children, onClick, disabled, loading }: any) => (
        <button disabled={disabled || loading} onClick={onClick}>
            {children}
        </button>
    ),
    Modal: (props: any) => {
        modal = props;
        React.useEffect(() => {
            props.afterOpenChange?.(props.open);
        }, [props.open]);
        return props.open ? (
            <div role="dialog">
                {props.children}
                {props.footer}
            </div>
        ) : null;
    },
}));
mock.module("@/components/ui/base/tooltip", () => ({ Tooltip: Box }));
mock.module("@/components/ui/product/media-placeholder", () => ({ MediaPlaceholder: () => <span>image placeholder</span> }));
let hallMounts = 0,
    galleryMounts = 0;
mock.module("@/pages/inspirations", () => ({
    default: ({ onUse }: any) => {
        React.useEffect(() => {
            hallMounts++;
        }, []);
        return <button onClick={() => onUse({ mode: "image", prompt: "Draft only" })}>hall choice</button>;
    },
}));
mock.module("@/pages/create/creation-inspiration-tunnel", () => ({
    CreationInspirationTunnel: ({ onStartPrompt }: any) => {
        React.useEffect(() => {
            galleryMounts++;
        }, []);
        return <button onClick={() => onStartPrompt("video", "Gallery draft")}>gallery choice</button>;
    },
}));
const { InspirationCover } = await import("@/pages/inspirations/inspiration-detail-modal");
const { InspirationEditorModal } = await import("@/pages/inspirations/inspiration-editor-modal");
const { CreationInspirationModules } = await import("@/pages/create/creation-inspiration-modules");
const host = document.createElement("div");
document.body.append(host);
let root = createRoot(host);
async function render(value: React.ReactNode) {
    await act(async () => {
        root.render(value);
    });
    await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0));
    });
}
function button(label: string) {
    const found = [...host.querySelectorAll("button")].find((button) => button.textContent === label);
    if (!found) throw new Error(`missing ${label}`);
    return found;
}
async function click(label: string) {
    await act(async () => {
        button(label).click();
    });
}
async function fresh() {
    await act(async () => {
        root.unmount();
        root = createRoot(host);
    });
}
function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (error: Error) => void;
    const promise = new Promise<T>((done, fail) => {
        resolve = done;
        reject = fail;
    });
    return { promise, resolve, reject };
}
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});

test("local covers render without a confirmation", async () => {
    await render(<InspirationCover image="/inspiration-thumbs/local.webp" />);
    expect(host.querySelector("img")?.getAttribute("src")).toBe("/inspiration-thumbs/local.webp");
});
test("external covers use native lazy loading without a click", async () => {
    await render(<InspirationCover image="https://images.example/one.webp" />);
    expect(host.querySelector("img")?.getAttribute("src")).toBe("https://images.example/one.webp");
    expect(host.querySelector("img")?.getAttribute("loading")).toBe("lazy");
    expect(host.querySelector("img")?.getAttribute("decoding")).toBe("async");
    expect(host.querySelector("img")?.getAttribute("referrerpolicy")).toBe("no-referrer");
    expect(host.querySelector("button")).toBeNull();
});
test("a new external address remains lazy without another action", async () => {
    await render(<InspirationCover image="https://images.example/two.webp" />);
    expect(host.querySelector("img")?.getAttribute("src")).toBe("https://images.example/two.webp");
    expect(host.querySelector("img")?.getAttribute("loading")).toBe("lazy");
    expect(host.querySelector("button")).toBeNull();
});
test("invalid image URLs never get a load action", async () => {
    for (const image of ["javascript:alert(1)", "//images.example/one", "https://user:password@images.example/one", "http://images.example/one"]) {
        await render(<InspirationCover image={image} />);
        expect(host.querySelector("img")).toBeNull();
        expect(host.querySelector("button")).toBeNull();
    }
});
test("failed images return to a placeholder", async () => {
    await render(<InspirationCover image="/inspiration-thumbs/failure.webp" />);
    await act(async () => {
        host.querySelector("img")!.dispatchEvent(new dom.Event("error") as any);
    });
    expect(host.querySelector("img")).toBeNull();
    expect(host.textContent).toContain("image placeholder");
});
test("closed optional modules never mount their lazy content", async () => {
    useUserStore.setState({ user: { id: "a", role: "user" } as any, features: { ...defaultFeatureAvailability } });
    hallMounts = 0;
    galleryMounts = 0;
    await render(
        <CreationInspirationModules
            mode="image"
            onStartPrompt={() => {
                throw new Error("unexpected use");
            }}
        />,
    );
    expect(hallMounts).toBe(0);
    expect(galleryMounts).toBe(0);
    expect(host.textContent).toBe("");
});
test("enabled hall and gallery only return visible drafts when clicked", async () => {
    const drafts: any[] = [];
    useUserStore.setState({ features: { ...defaultFeatureAvailability, inspirationHallEnabled: true, inspirationGalleryEnabled: true } });
    await render(<CreationInspirationModules mode="image" onStartPrompt={(...value) => drafts.push(value)} />);
    expect(drafts).toEqual([]);
    await click("hall choice");
    await click("gallery choice");
    expect(drafts).toEqual([
        ["image", "Draft only"],
        ["video", "Gallery draft"],
    ]);
});
test("editor retains its data on a failed server save", async () => {
    await fresh();
    let closed = 0;
    notices.length = 0;
    await render(
        <InspirationEditorModal
            open
            item={null}
            categoryOptions={[]}
            onClose={() => closed++}
            onSubmit={async () => {
                throw new Error("server save failed");
            }}
        />,
    );
    form.setFieldsValue({ title: "Keep", description: "", image: "", mode: "image", prompt: "Keep body" });
    await click("创建");
    expect(closed).toBe(0);
    expect(form.values.prompt).toBe("Keep body");
    expect(notices).toContain("server save failed");
});
test("editor stays open and blocks rapid callbacks until receipt", async () => {
    await fresh();
    let closed = 0,
        writes = 0;
    const pending = deferred<void>();
    await render(
        <InspirationEditorModal
            open
            item={null}
            categoryOptions={[]}
            onClose={() => closed++}
            onSubmit={async () => {
                writes++;
                await pending.promise;
            }}
        />,
    );
    form.setFieldsValue({ title: "Keep", description: "", image: "", mode: "image", prompt: "Keep body" });
    await act(async () => {
        button("创建").click();
        button("创建").click();
    });
    expect(writes).toBe(1);
    expect(closed).toBe(0);
    pending.resolve();
    await act(async () => {
        await pending.promise;
    });
    expect(closed).toBe(1);
});
test("owned cover references survive editing without URL substitution", async () => {
    await fresh();
    let saved: any;
    const item = { id: "custom:old-id", title: "Old", description: "", image: "/api/resources/owned/file", coverResourceId: "owned", mode: "image" as const, prompt: "Original" };
    await render(
        <InspirationEditorModal
            open
            item={item}
            categoryOptions={[]}
            onClose={() => {}}
            onSubmit={async (draft) => {
                saved = draft;
            }}
        />,
    );
    await click("保存");
    expect(saved.coverResourceId).toBe("owned");
    expect(saved.prompt).toBe("Original");
});
