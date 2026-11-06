import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";

const dom = new Window({ url: "http://localhost/canvas/private-id?token=secret#private" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true })) {
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { useUserStore, defaultFeatureAvailability } = await import("@/stores/use-user-store");
const notices: string[] = [];
const fields = new Map<string, any>();
let checkbox: any;
const Box = ({ children }: any) => <div>{children}</div>;
const Overlay = ({ open, children }: any) => (open ? <div role="dialog">{children}</div> : null);
const Input = (props: any) => {
    fields.set(props.placeholder || "reply", props);
    return <input value={props.value || ""} onChange={props.onChange} placeholder={props.placeholder} />;
};
Input.TextArea = Input;
const appApi = { message: { success: (text: string) => notices.push(text), error: (text: string) => notices.push(text), warning: (text: string) => notices.push(text) } };
mock.module("antd", () => ({
    App: { useApp: () => appApi },
    Input,
    Modal: Overlay,
    Drawer: Overlay,
    Radio: Object.assign(Box, { Group: Box, Button: Box }),
    Tag: Box,
    Tooltip: Box,
    Image: (props: any) => <img src={props.src} alt={props.alt} />,
    Button: ({ children, onClick, disabled, loading, id }: any) => (
        <button id={id} disabled={disabled || loading} onClick={onClick}>
            {children}
        </button>
    ),
    Checkbox: (props: any) => {
        checkbox = props;
        return <input type="checkbox" checked={props.checked} onChange={props.onChange} />;
    },
    Segmented: ({ options, onChange }: any) => (
        <div>
            {options.map((option: any) => (
                <button key={option.value} onClick={() => onChange(option.value)}>
                    {option.label}
                </button>
            ))}
        </div>
    ),
    Select: () => null,
    Space: Box,
    Popconfirm: ({ children, onConfirm }: any) => (
        <span>
            <button onClick={onConfirm}>confirm archive</button>
            {children}
        </span>
    ),
    Table: ({ dataSource, columns }: any) => (
        <div>
            {dataSource.map((item: any) => (
                <div key={item.id}>
                    {columns.map((column: any, index: number) => (
                        <div key={index}>{column.render?.(item[column.dataIndex], item)}</div>
                    ))}
                </div>
            ))}
        </div>
    ),
}));
mock.module("@/components/ui/product/app-modal", () => ({ AppModal: Overlay }));
mock.module("@/pages/admin/components/admin-shell", () => ({ AdminPageFrame: Box }));
mock.module("@/services/api/admin-storage", () => ({ adminResourceFileUrl: (id: string) => `/api/admin/resources/${id}/file` }));
let submissions: any[] = [];
let adminWrites: any[] = [];
let submitImpl = async () => ({ id: "created" });
let mineImpl: () => Promise<any[]> = async () => [];
let adminImpl: () => Promise<any>;
let updateImpl: () => Promise<any>;
mock.module("@/services/api/feedback", () => ({
    submitFeedback: async (payload: any) => {
        submissions.push(payload);
        return submitImpl();
    },
    getMyFeedbacks: () => mineImpl(),
    uploadFeedbackAttachment: async () => ({ id: "owned", url: "/api/resources/owned/file", type: "image", name: "shot.png" }),
    getAdminFeedbacks: () => adminImpl(),
    updateAdminFeedback: async (...args: any[]) => {
        adminWrites.push(args);
        return updateImpl();
    },
    archiveAdminFeedback: async (...args: any[]) => {
        adminWrites.push(args);
        return updateImpl();
    },
}));
const { FeedbackModal } = await import("@/components/feedback/feedback-modal");
const { FeedbackInboxPane } = await import("@/pages/settings/feedback-inbox-pane");
const { default: AdminFeedbacksPage } = await import("@/pages/admin/feedbacks/feedbacks-page");
const host = document.createElement("div");
document.body.append(host);
let root = createRoot(host);
async function render(value: React.ReactNode) {
    await act(async () => {
        root.render(value);
    });
}
async function click(label: string) {
    const target = [...host.querySelectorAll("button")].findLast((button) => button.textContent?.trim() === label);
    if (!target) throw new Error(`missing ${label}`);
    await act(async () => target.click());
}
async function fresh(enabled = true) {
    await act(async () => {
        root.unmount();
        root = createRoot(host);
    });
    useUserStore.setState({ user: { id: "a", username: "fixture", role: "user" } as any, features: { ...defaultFeatureAvailability, userFeedbackEnabled: enabled } });
    fields.clear();
    notices.length = 0;
    submissions = [];
    adminWrites = [];
    submitImpl = async () => ({ id: "created" });
    mineImpl = async () => [];
}
async function fill() {
    await act(async () => {
        [...fields.entries()].find(([name]) => name.startsWith("例如："))![1].onChange({ target: { value: "Keep title" } });
        [...fields.entries()].find(([name]) => name.startsWith("例如：在画布里"))![1].onChange({ target: { value: "Keep body" } });
    });
}
const feedback = { id: "fb", title: "Feedback", content: "body", userName: "User", status: "pending", type: "bug", createdAt: "2026-10-26" };
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});

test("official diagnostics omit route secrets and can be excluded explicitly", async () => {
    await fresh();
    await render(<FeedbackModal open onClose={() => {}} />);
    await fill();
    submitImpl = async () => {
        throw new Error("retry later");
    };
    await click("提交反馈");
    const environment = JSON.parse(submissions[0].environment);
    expect(environment.pathname).toBe("/canvas/:id");
    expect(environment.currentUrl).toBeUndefined();
    expect(JSON.stringify(environment)).not.toContain("secret");
    expect(JSON.stringify(environment)).not.toContain("private-id");
    await act(async () => checkbox.onChange({ target: { checked: false } }));
    await click("提交反馈");
    expect(submissions[1].environment).toBe("{}");
    expect(submissions[1]).not.toHaveProperty("requestId");
    expect([...fields.values()].some((props) => props.value === "Keep body")).toBe(true);
    expect(notices).toContain("retry later");
});

test("official submission clears the form after acknowledgement and opens history", async () => {
    await fresh();
    mineImpl = async () => [{ ...feedback, title: "Submitted title" }];
    await render(<FeedbackModal open onClose={() => {}} />);
    await fill();
    await click("提交反馈");
    expect(submissions[0]).toMatchObject({ type: "bug", title: "Keep title", content: "Keep body", attachments: "[]" });
    expect(host.textContent).toContain("Submitted title");
    const submitTab = host.querySelector<HTMLButtonElement>('[role="radio"]');
    expect(submitTab).not.toBeNull();
    await act(async () => submitTab!.click());
    expect([...fields.values()].some((props) => props.value === "Keep body")).toBe(false);
});

test("disabled submission keeps existing feedback history readable", async () => {
    await fresh(false);
    mineImpl = async () => [{ ...feedback, title: "Existing history" }];
    await render(<FeedbackModal open onClose={() => {}} />);
    expect([...host.querySelectorAll<HTMLButtonElement>("button")].findLast((button) => button.textContent?.trim() === "提交反馈")!.disabled).toBe(true);
    await render(<FeedbackInboxPane />);
    expect(host.textContent).toContain("Existing history");
});

test("admin uses the official reply contract and preserves a failed draft", async () => {
    await fresh();
    adminImpl = async () => ({ list: [feedback], total: 1 });
    updateImpl = async () => {
        throw new Error("reply unavailable");
    };
    await render(<AdminFeedbacksPage />);
    const detail = [...host.querySelectorAll("button")].find((button) => button.textContent?.includes("详情"))!;
    await act(async () => detail.click());
    await act(async () => [...fields.entries()].find(([name]) => name.startsWith("输入回复内容"))![1].onChange({ target: { value: "Keep reply" } }));
    await click("保存并答复");
    expect(adminWrites[0]).toEqual(["fb", { status: "pending", adminReply: "Keep reply" }]);
    expect([...fields.values()].some((props) => props.value === "Keep reply")).toBe(true);
    expect(notices).toContain("reply unavailable");
});
