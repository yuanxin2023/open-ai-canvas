import { afterAll, expect, mock, test } from "bun:test";
import { Window } from "happy-dom";
const dom = new Window({ url: "http://localhost/canvas/private-id?token=secret#private" });
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, Event: dom.Event, IS_REACT_ACT_ENVIRONMENT: true }))
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
const React = await import("react");
const { act } = React;
const { createRoot } = await import("react-dom/client");
const { useUserStore, defaultFeatureAvailability } = await import("@/stores/use-user-store");
const notices: string[] = [];
const fields = new Map<string, any>();
let checkbox: any;
const Box = ({ children }: any) => <div>{children}</div>;
const Input = (props: any) => {
    fields.set(props.placeholder || "reply", props);
    return <input value={props.value || ""} onChange={props.onChange} placeholder={props.placeholder} />;
};
Input.TextArea = Input;
mock.module("antd", () => ({
    App: { useApp: () => ({ message: { success: (text: string) => notices.push(text), error: (text: string) => notices.push(text), warning: (text: string) => notices.push(text) } }) },
    Input,
    Radio: Box,
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
    Segmented: () => null,
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
mock.module("@/components/ui/product/app-modal", () => ({ AppModal: ({ open, children }: any) => (open ? <div role="dialog">{children}</div> : null) }));
mock.module("@/components/ui/product/app-drawer", () => ({ AppDrawer: ({ open, children }: any) => (open ? <div role="dialog">{children}</div> : null) }));
mock.module("@/pages/admin/components/admin-shell", () => ({ AdminPageFrame: Box }));
mock.module("@/services/api/resources", () => ({ resourceFileUrl: (id: string) => `/api/resources/${id}/file` }));
let submissions: any[] = [];
let adminWrites: any[] = [];
let submitImpl: () => Promise<any> = async () => ({ id: "created" });
let mineImpl: () => Promise<any[]> = async () => [];
let adminImpl: () => Promise<any>;
let updateImpl: () => Promise<any>;
mock.module("@/services/api/feedback", () => ({
    submitFeedback: async (payload: any, actorId: string) => {
        submissions.push({ payload, actorId });
        return submitImpl();
    },
    getMyFeedbacks: () => mineImpl(),
    uploadFeedbackAttachment: async () => ({ id: "owned" }),
    feedbackAttachmentURL: (id: string, resource: string) => `/api/feedbacks/${id}/attachments/${resource}`,
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
    const target = [...host.querySelectorAll("button")].find((button) => button.textContent?.trim() === label);
    if (!target) throw new Error(`missing ${label}`);
    await act(async () => {
        target.click();
    });
}
function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((done) => {
        resolve = done;
    });
    return { promise, resolve };
}
async function fresh(enabled = true, admin = false) {
    await act(async () => {
        root.unmount();
        root = createRoot(host);
    });
    useUserStore.setState({ user: { id: "a", role: admin ? "admin" : "user", adminAccess: admin ? { level: "full", permissions: [] } : undefined } as any, features: { ...defaultFeatureAvailability, userFeedbackEnabled: enabled } });
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
        [...fields.entries()].find(([name]) => name.startsWith("请尽可能"))![1].onChange({ target: { value: "Keep body" } });
    });
}
afterAll(async () => {
    await act(async () => root.unmount());
    dom.happyDOM.abort();
});

test("closed feedback hides form and inbox without reading history", async () => {
    await fresh(false);
    let reads = 0;
    mineImpl = async () => {
        reads++;
        return [];
    };
    await render(
        <>
            <FeedbackModal open onClose={() => {}} />
            <FeedbackInboxPane />
        </>,
    );
    expect(host.textContent).toBe("");
    expect(reads).toBe(0);
});
test("diagnostics default to off and only explicit opt-in collects a masked route", async () => {
    await fresh();
    await render(<FeedbackModal open onClose={() => {}} />);
    expect(checkbox.checked).toBe(false);
    expect(host.querySelector("pre")).toBeNull();
    await act(async () => {
        checkbox.onChange({ target: { checked: true } });
    });
    const env = JSON.parse(host.querySelector("pre")!.textContent!);
    expect(env.pathname).toBe("/canvas/:id");
    expect(env.currentUrl).toBeUndefined();
    expect(JSON.stringify(env)).not.toContain("secret");
    expect(JSON.stringify(env)).not.toContain("private-id");
});
test("failure retains draft and retry reuses the account-scoped request", async () => {
    await fresh();
    submitImpl = async () => {
        throw new Error("unknown receipt");
    };
    await render(<FeedbackModal open onClose={() => {}} />);
    await fill();
    await click("提交反馈");
    await click("提交反馈");
    expect(submissions).toHaveLength(2);
    expect(submissions[0].actorId).toBe("a");
    expect(submissions[0].payload.requestId).toBe(submissions[1].payload.requestId);
    expect(submissions[0].payload.environment).toBe("{}");
    expect([...fields.values()].some((props) => props.value === "Keep body")).toBe(true);
    expect(notices).toContain("unknown receipt");
});
test("rapid submit callbacks issue one write until acknowledgement", async () => {
    await fresh();
    const pending = deferred<any>();
    submitImpl = () => pending.promise;
    await render(<FeedbackModal open onClose={() => {}} />);
    await fill();
    const button = host.querySelector<HTMLButtonElement>("#feedback-submit-btn")!;
    await act(async () => {
        button.click();
        button.click();
    });
    expect(submissions).toHaveLength(1);
    pending.resolve({ id: "created" });
    await act(async () => {
        await pending.promise;
    });
    expect(notices).toContain("反馈已提交");
});
test("changed draft requires explicit confirmation after an unknown receipt", async () => {
    await fresh();
    submitImpl = async () => {
        throw new Error("unknown receipt");
    };
    await render(<FeedbackModal open onClose={() => {}} />);
    await fill();
    await click("提交反馈");
    await act(async () => {
        [...fields.entries()].find(([name]) => name.startsWith("请尽可能"))![1].onChange({ target: { value: "changed body" } });
    });
    await click("提交反馈");
    expect(submissions).toHaveLength(1);
    await click("已核对，作为新反馈");
    await click("提交反馈");
    expect(submissions).toHaveLength(2);
    expect(submissions[0].payload.requestId).not.toBe(submissions[1].payload.requestId);
});
test("late submission cannot clear or notify a different account", async () => {
    await fresh();
    const pending = deferred<any>();
    submitImpl = () => pending.promise;
    await render(<FeedbackModal open onClose={() => {}} />);
    await fill();
    await click("提交反馈");
    await act(async () => {
        useUserStore.setState({ user: { id: "b", role: "user" } as any });
    });
    pending.resolve({ id: "created" });
    await act(async () => {
        await pending.promise;
    });
    expect(notices).not.toContain("反馈已提交");
    expect([...fields.values()].some((props) => props.value === "Keep body")).toBe(false);
});
test("inbox reports read failure and rejects a late response for another account", async () => {
    await fresh();
    mineImpl = async () => {
        throw new Error("history unavailable");
    };
    await render(<FeedbackInboxPane />);
    expect(host.querySelector('[role="alert"]')?.textContent).toContain("history unavailable");
    await fresh();
    const pending = deferred<any[]>();
    mineImpl = () => pending.promise;
    await render(<FeedbackInboxPane />);
    mineImpl = async () => [];
    await act(async () => {
        useUserStore.setState({ user: { id: "b", role: "user" } as any });
    });
    pending.resolve([{ id: "private", title: "Private feedback", content: "secret", type: "bug", status: "pending", createdAt: "2026-10-26" }]);
    await act(async () => {
        await pending.promise;
    });
    // The replacement account's read is independent; resolve only the old request.
    expect(host.textContent).not.toContain("Private feedback");
});
test("admin saves captured version and keeps draft after a conflict", async () => {
    await fresh(true, true);
    adminImpl = async () => ({ list: [{ id: "fb", version: 7, title: "Feedback", content: "body", userName: "User", status: "pending", type: "bug", createdAt: "2026-10-26" }], total: 1 });
    updateImpl = async () => {
        throw new Error("feedback changed");
    };
    await render(<AdminFeedbacksPage />);
    const detail = [...host.querySelectorAll("button")].find((button) => button.textContent?.includes("详情"))!;
    await act(async () => {
        detail.click();
    });
    await act(async () => {
        fields.get("输入回复内容，例如：已定位问题并将在下一版本发布修复；或者感谢提议，该需求已排入排期计划...").onChange({ target: { value: "Keep reply" } });
    });
    await click("保存并答复");
    expect(adminWrites[0][2]).toBe("a");
    expect(adminWrites[0][3]).toBe(7);
    expect([...fields.values()].some((props) => props.value === "Keep reply")).toBe(true);
    expect(notices).toContain("feedback changed");
});
