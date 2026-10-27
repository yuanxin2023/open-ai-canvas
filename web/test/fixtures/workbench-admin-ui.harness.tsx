import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
import type { WorkbenchTool } from "@/services/api/workbench";

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
const { useUserStore } = await import("@/stores/use-user-store");
const realAPI = await import("@/services/api/workbench");
let form: any;
let modal: any;
let reads: Array<{ kind: string; signal?: AbortSignal; admin: boolean }> = [];
let writes: any[] = [];
let changes: any[] = [];
let notices: string[] = [];
let list: (kind: string) => Promise<{ tools: WorkbenchTool[] }>;
let save: (input: any) => Promise<WorkbenchTool>;
let publish: () => Promise<boolean>;
let resolveMedia: () => Promise<string | undefined>;
let validate: () => Promise<void>;
const app = { message: { success: (text: string) => notices.push(text), error: (text: string) => notices.push(text), warning: (text: string) => notices.push(text) } };
const Box = ({ children }: any) => <div>{children}</div>;
const Form = Object.assign(Box, {
    Item: Box,
    useForm: () => {
        const current = React.useRef<any>(null);
        if (!current.current)
            current.current = {
                values: {},
                resetFields() {
                    this.values = {};
                },
                setFieldsValue(values: any) {
                    Object.assign(this.values, values);
                },
                setFieldValue(key: string, value: any) {
                    this.values[key] = value;
                },
                async validateFields() {
                    await validate();
                    return { ...this.values };
                },
            };
        form = current.current;
        return [form];
    },
});
const Input = Object.assign(({ value = "", onChange, "aria-label": label }: any) => <input aria-label={label} value={value} onChange={onChange} />, {
    TextArea: ({ value = "", onChange, "aria-label": label }: any) => <textarea aria-label={label} value={value} onChange={onChange} />,
});
mock.module("antd", () => ({
    App: { useApp: () => app },
    Form,
    Input,
    Upload: Box,
    InputNumber: () => null,
    Button: ({ children, onClick, disabled, "aria-label": label }: any) => (
        <button aria-label={label} disabled={disabled} onClick={onClick}>
            {children}
        </button>
    ),
    Switch: ({ checked = false, disabled, onChange, "aria-label": label }: any) => <input type="checkbox" aria-label={label} checked={checked} disabled={disabled} onChange={(event) => onChange?.(event.target.checked)} />,
    Select: ({ value, options, onChange, disabled, "aria-label": label }: any) =>
        label ? (
            <select aria-label={label} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
                {options.map((option: any) => (
                    <option key={option.value} value={option.value}>
                        {option.label}
                    </option>
                ))}
            </select>
        ) : null,
    Modal: (props: any) => {
        modal = props;
        return props.open ? (
            <section role="dialog" className={props.className}>
                <h2>{props.title}</h2>
                {props.children}
                <button disabled={props.confirmLoading || props.okButtonProps?.disabled} onClick={props.onOk}>
                    保存
                </button>
            </section>
        ) : null;
    },
}));
mock.module("@/pages/admin/components/admin-shell", () => ({
    AdminPageFrame: ({ title, children, actions }: any) => (
        <main>
            <h1>{title}</h1>
            {actions}
            {children}
        </main>
    ),
}));
mock.module("@/pages/admin/components/admin-ui", () => ({
    AdminStatusBadge: ({ label }: any) => <span>{label}</span>,
    AdminDataTable: ({ table, empty }: any) =>
        table.dataSource.length ? (
            <div>
                {table.dataSource.map((row: any) => (
                    <article key={row.id}>
                        {table.columns.map((column: any, index: number) => (
                            <div key={index}>{column.render ? column.render(row[column.dataIndex], row) : row[column.dataIndex]}</div>
                        ))}
                    </article>
                ))}
            </div>
        ) : table.loading ? (
            <span>loading</span>
        ) : (
            empty
        ),
}));
mock.module("@/services/api/workbench", () => ({
    ...realAPI,
    listWorkbenchTools: async (kind: string, signal: AbortSignal, admin: boolean) => {
        reads.push({ kind, signal, admin });
        return list(kind);
    },
    saveAdminWorkbenchTool: async (input: any) => {
        writes.push(input);
        return save(input);
    },
    setAdminWorkbenchToolEnabled: async (id: string, enabled: boolean) => {
        changes.push({ id, enabled });
        return publish();
    },
    resolveWorkbenchResourceURL: () => resolveMedia(),
}));
mock.module("@/services/api/resources", () => ({
    uploadResourceFile: async () => {
        throw new Error("unexpected upload");
    },
}));
const { default: WorkbenchToolsPage } = await import("@/pages/admin/workbench-tools/workbench-tools-page");
const tool: WorkbenchTool = {
    id: "builtin-tool",
    kind: "image",
    categoryId: "storyboard",
    name: "分镜参考",
    subtitle: "同一角色",
    description: "可审核的场景模板",
    enabled: false,
    sortOrder: 1000,
    presets: [{ id: "builtin-preset", name: "标准版", summary: "", promptTemplate: "A storyboard", enabled: false, defaultParameters: { size: "16:9" } }],
};
const host = document.createElement("div");
document.body.append(host);
let root: ReturnType<typeof createRoot> | undefined;
const tick = () => new Promise((resolve) => setTimeout(resolve, 0));
async function settle() {
    await act(async () => {
        await tick();
        await tick();
    });
}
function actor(id = "admin-a", permissions = ["admin.platform.workbench_tools"]) {
    return { id, role: "admin", adminAccess: { level: "scoped", permissions } } as any;
}
async function mount(user = actor()) {
    await act(async () => {
        root?.unmount();
    });
    reads = [];
    writes = [];
    changes = [];
    notices = [];
    modal = null;
    list = async () => ({ tools: [structuredClone(tool)] });
    save = async (input) => ({ ...tool, ...input });
    publish = async () => true;
    resolveMedia = async () => undefined;
    validate = async () => {};
    useUserStore.setState({ user });
    await act(async () => {
        root = createRoot(host);
        root.render(<WorkbenchToolsPage />);
    });
    await settle();
}
function button(label: string) {
    const found = [...host.querySelectorAll("button")].find((item) => item.textContent?.trim() === label);
    if (!found) throw new Error(`missing button ${label}`);
    return found;
}
async function click(label: string) {
    await act(async () => {
        button(label).click();
    });
    await settle();
}
async function clickCheckbox(label: string) {
    await act(async () => {
        (host.querySelector(`input[aria-label="${label}"]`) as HTMLInputElement).click();
    });
    await settle();
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
    await act(async () => {
        root?.unmount();
    });
    dom.happyDOM.abort();
});

test("ungranted scoped administrators cannot load or edit tools", async () => {
    await mount(actor("admin-a", ["admin.platform.channels", "admin.platform.skills"]));
    expect(reads).toHaveLength(0);
    expect(host.textContent).toContain("没有工作台工具管理权限");
    expect(host.querySelectorAll("button")).toHaveLength(0);
});

test("new tools and new prompt presets stay disabled until explicitly selected", async () => {
    await mount();
    await click("新建工具");
    await click("添加预设");
    expect(form.values.enabled).toBe(false);
    expect((host.querySelector('input[aria-label="启用预设 1"]') as HTMLInputElement).checked).toBe(false);
    form.setFieldsValue({ name: "New", description: "Description" });
    await click("保存");
    expect(writes).toHaveLength(0);
    expect(notices).toContain("请补齐预设名称和提示词");
});

test("saving existing content preserves disabled states, preset IDs and parameters", async () => {
    await mount();
    await click("编辑");
    await click("保存");
    expect(writes).toHaveLength(1);
    expect(writes[0]).toMatchObject({ id: tool.id, enabled: false, categoryId: tool.categoryId, presets: [{ id: "builtin-preset", enabled: false, defaultParameters: { size: "16:9" } }] });
    expect(reads.every((read) => read.admin && read.signal instanceof AbortSignal)).toBe(true);
});

test("editing previously published content does not disable it", async () => {
    await mount();
    list = async () => ({ tools: [{ ...tool, enabled: true, presets: [{ ...tool.presets[0], enabled: true }] }] });
    await click("刷新");
    await click("编辑");
    await click("保存");
    expect(writes[0].enabled).toBe(true);
    expect(writes[0].presets[0].enabled).toBe(true);
});

test("save failure retains the editor and all disabled states", async () => {
    await mount();
    await click("编辑");
    save = async () => {
        throw new Error("server denied save");
    };
    await click("保存");
    expect(host.querySelector('[role="dialog"]')).not.toBeNull();
    expect(form.values.enabled).toBe(false);
    expect(notices).toContain("server denied save");
    expect(notices.some((notice) => notice.includes("已保存"))).toBe(false);
});

test("rapid save callbacks create only one write", async () => {
    await mount();
    await click("编辑");
    const wait = deferred<WorkbenchTool>();
    save = () => wait.promise;
    await act(async () => {
        modal.onOk();
        modal.onOk();
    });
    await settle();
    expect(writes).toHaveLength(1);
    wait.resolve(tool);
    await settle();
});

test("old directory results cannot replace the newly selected kind", async () => {
    await mount();
    const wait = deferred<{ tools: WorkbenchTool[] }>();
    list = (kind) => (kind === "image" ? wait.promise : Promise.resolve({ tools: [{ ...tool, id: "video-tool", kind: "video", name: "视频模板" }] }));
    await click("刷新");
    await act(async () => {
        const select = host.querySelector('select[aria-label="工作台类型"]') as HTMLSelectElement;
        select.value = "video";
        select.dispatchEvent(new dom.Event("change", { bubbles: true }) as any);
    });
    await settle();
    wait.resolve({ tools: [tool] });
    await settle();
    expect(host.textContent).toContain("视频模板");
    expect(host.textContent).not.toContain("分镜参考");
});

test("an account switch prevents a delayed editor from opening", async () => {
    await mount();
    const wait = deferred<string | undefined>();
    resolveMedia = () => wait.promise;
    await click("编辑");
    await act(async () => {
        useUserStore.setState({ user: actor("admin-b", []) });
    });
    wait.resolve("https://media.example/old-account");
    await settle();
    expect(host.querySelector('[role="dialog"]')).toBeNull();
    expect(host.textContent).toContain("没有工作台工具管理权限");
});

test("an account switch during validation prevents a save", async () => {
    await mount();
    await click("编辑");
    const wait = deferred<void>();
    validate = () => wait.promise;
    await act(async () => {
        modal.onOk();
    });
    await act(async () => {
        useUserStore.setState({ user: actor("admin-b") });
    });
    wait.resolve();
    await settle();
    expect(writes).toHaveLength(0);
});

test("publication failures propagate and duplicate toggles remain blocked", async () => {
    await mount();
    const wait = deferred<boolean>();
    publish = () => wait.promise;
    await clickCheckbox("开放分镜参考");
    await clickCheckbox("开放分镜参考");
    expect(changes).toEqual([{ id: tool.id, enabled: true }]);
    wait.reject(new Error("publication rejected"));
    await settle();
    expect(notices).toContain("publication rejected");
    expect((host.querySelector('input[aria-label="开放分镜参考"]') as HTMLInputElement).checked).toBe(false);
    list = async () => {
        throw new Error("directory unavailable");
    };
    await click("刷新");
    expect(host.querySelector('[role="alert"]')?.textContent).toBe("directory unavailable");
    expect(host.textContent).not.toContain("当前类型还没有工具");
});
