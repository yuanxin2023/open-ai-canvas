import { expect, test } from "bun:test";

test("creation thread exposes a persistent desktop conversation sidebar", async () => {
    const [pageSource, workspaceSource, sidebarSource, shellSource, styles, globalStyles] = await Promise.all([
        Bun.file(new URL("../src/pages/create/index.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/create/creation-workspace.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/components/layout/workspace-sidebar-nav.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/components/layout/app-top-nav.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/create/creation-product.css", import.meta.url)).text(),
        Bun.file(new URL("../src/styles/globals.css", import.meta.url)).text(),
    ]);

    expect(pageSource).toContain("<CreationConversationSidebar");
    expect(pageSource).toContain("conversations={historyConversations}");
    expect(pageSource).toContain("showThreadWorkspace");
    expect(pageSource).toContain("CONVERSATION_SIDEBAR_OPEN_PREF_KEY");
    expect(pageSource).toContain("conversationSidebarOpen");
    expect(pageSource).toContain("const enteringThreadFromHome = !showThreadWorkspace");
    expect(pageSource).toContain("setConversationSidebarOpen(true)");
    expect(pageSource).toContain("publishWorkspaceSidebarCollapsed(true)");
    expect(shellSource).toContain("subscribeWorkspaceSidebarCollapsed(setDesktopSidebarCollapsed)");
    expect(pageSource).toContain("onCollapse={() => setConversationSidebarOpen(false)}");
    expect(pageSource).toContain("onDelete={confirmDeleteConversation}");
    expect(pageSource).toContain("onRename={renameConversationTitle}");
    expect(pageSource).toContain("onTogglePin={toggleConversationPin}");
    expect(pageSource).not.toContain("CreationHistoryDrawer");
    expect(pageSource).not.toContain("historyOpen");
    expect(pageSource).not.toContain("onOpenHistory");
    expect(pageSource).toContain('aria-label="查看创作对话"');
    expect(pageSource).toContain("setConversationSidebarOpen(true)");
    expect(pageSource).toContain("setInspirationHomeOpen(false)");
    expect(workspaceSource).not.toContain("export function CreationHistoryDrawer");
    expect(workspaceSource).not.toContain("creation-history-drawer");
    expect(pageSource).toContain("sort(compareCreationConversations)");
    expect(pageSource).toContain("creationConversationTitle(text, mode)");
    expect(pageSource).toContain('aria-label="展开创作对话"');
    expect(pageSource).toContain("current.filter((conversation) => conversation.messages.length > 0)");
    expect(pageSource).toContain("inspirationHomeOpen");
    expect(pageSource).toContain('new URLSearchParams(location.search).get("home") !== "1"');
    expect(pageSource).toContain("openInspirationHome()");
    expect(pageSource).toContain('navigate("/", { replace: true })');
    expect(sidebarSource).toContain('{ ...toolItem("create", "/?home=1"), id: "home", title: "创作" }');
    expect(sidebarSource).toContain('to="/?home=1"');
    expect(pageSource).toContain('className="creation-thread-main"');
    expect(workspaceSource).toContain("export function CreationConversationSidebar");
    expect(workspaceSource).toContain("新建对话");
    expect(workspaceSource).toContain('aria-label="收起创作对话"');
    expect(workspaceSource).toContain('aria-label="历史对话"');
    expect(workspaceSource).toContain('aria-label="搜索创作对话"');
    expect(workspaceSource).toContain("filterCreationConversations");
    expect(workspaceSource).toContain("creationConversationBucket(conversation.updatedAt)");
    expect(workspaceSource).toContain("conversationPreviewImage(conversation)");
    expect(workspaceSource).toContain("onError={() => setFailed(true)}");
    expect(workspaceSource).toContain('label: "重命名"');
    expect(workspaceSource).toContain('label: "导出对话"');
    expect(workspaceSource).toContain('label: "删除对话"');
    expect(workspaceSource).toContain('label: conversation.pinned ? "取消置顶" : "置顶对话"');
    expect(workspaceSource).toContain('pinned: "置顶"');
    expect(workspaceSource).toContain("creationConversationDisplayTitle(conversation)");
    expect(workspaceSource).toContain("state.features.shortDramaEnabled");
    expect(workspaceSource).toContain('shortDramaEnabled ? <div className="creation-toolbar-shots"');
    expect(styles).toContain(".creation-conversation-sidebar");
    expect(styles).toContain("width: clamp(304px, 22vw, 336px)");
    expect(styles).toContain("background: var(--user-context-panel-bg, var(--user-surface))");
    expect(styles).toContain("border-right: 1px solid var(--user-panel-divider, var(--user-border))");
    expect(styles).toContain("background: var(--user-context-panel-selected, var(--user-surface-muted))");
    expect(styles).toContain("margin: 4px -8px 0 0; padding-right: 8px;");
    expect(styles).toContain(".creation-conversation-sidebar-search");
    expect(styles).toContain(".creation-conversation-sidebar-group");
    expect(styles).toContain(".creation-conversation-sidebar-more");
    expect(styles).toContain(".creation-conversation-sidebar-rename");
    expect(styles).toContain(".creation-conversation-sidebar-expand");
    expect(styles).toContain(".creation-thread-empty-state");
    expect(styles).toContain("@media (max-width: 1180px)");
    expect(styles).toContain("width: min(336px, 100%)");
    expect(styles).toContain("inset: 0 auto 0 0");
    expect(styles).toContain(".creation-toolbar-conversation-actions");
    expect(globalStyles).toContain(".creation-top-actions");
    expect(globalStyles).toContain("left: 28px;");
    expect(globalStyles).toContain(".creation-top-actions { top: 14px; left: 14px; }");
    expect(globalStyles).toContain(".creation-top-action:disabled");
});
