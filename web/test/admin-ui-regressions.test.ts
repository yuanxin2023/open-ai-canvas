import { expect, test } from "bun:test";

function compactSource(source: string) {
    return source.replace(/\s+/g, " ").trim();
}

function sourceSection(source: string, startMarker: string, endMarker: string) {
    const start = source.indexOf(startMarker);
    const end = source.indexOf(endMarker, start + startMarker.length);
    expect(start).toBeGreaterThanOrEqual(0);
    expect(end).toBeGreaterThan(start);
    return source.slice(start, end);
}

test("announcement editor preserves image and pinned fields through edit and save", async () => {
    const [panelSource, safetySource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/components/admin-announcements-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/admin-announcement-safety.ts", import.meta.url)).text(),
    ]);
    const panel = compactSource(panelSource);

    expect(panel).toContain("uploadAdminAnnouncementImage");
    expect(panel).toContain("discardAdminAnnouncementImage");
    expect(panel).toContain('imageResourceId: announcement.imageResourceId || ""');
    expect(panel).toContain("pinned: announcement.pinned");
    expect(panel).toContain('imageResourceId: values.imageResourceId?.trim() || ""');
    expect(panel).toContain("pinned: Boolean(values.pinned)");
    expect(panel).toContain('(announcement?.imageResourceId || "") === (expectedContent.imageResourceId || "")');
    expect(panel).toContain('rootClassName="admin-modal-root admin-announcement-editor-modal"');
    expect(panel).toContain("centered");
    expect(panel).not.toContain("<Drawer");
    expect(safetySource).toContain("imageResourceId?: string");
    expect(safetySource).toContain("pinned?: boolean");
});

test("plugin upload owns native drops and price availability text remains readable", async () => {
    const [pluginSource, adminCss] = await Promise.all([Bun.file(new URL("../src/pages/plugins/plugin-documentation-modals.tsx", import.meta.url)).text(), Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text()]);
    const toggleCss = sourceSection(adminCss, ".admin-price-tier-toggle span {", ".admin-model-editor-add-tier.ant-btn {");

    expect(pluginSource).toContain("event.preventDefault()");
    expect(pluginSource).toContain("onDragOver={(event)");
    expect(pluginSource).toContain("onDrop={handlePluginDrop}");
    expect(pluginSource).toContain("点击选择插件文件，也可拖拽到此处");
    expect(pluginSource).toContain("释放文件以上传插件");
    expect(pluginSource).toContain("isDraggingPlugin");
    expect(compactSource(adminCss)).toContain(".admin-price-tier-controls { margin-left: auto;");
    expect(toggleCss).toContain("overflow-wrap: anywhere;");
    expect(toggleCss).toContain("white-space: normal;");
    expect(toggleCss).not.toContain("text-overflow: ellipsis;");
});

test("model reference limits use compact rows only inside the admin editor", async () => {
    const css = await Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text();
    const numberField = sourceSection(css, ".admin-model-editor-references .admin-capability-number-field {", ".admin-model-editor-references .admin-capability-boolean-field {");
    expect(numberField).toContain("grid-template-columns: minmax(0, 1fr) 80px;");
    expect(numberField).toContain("min-height: 32px;");
    expect(numberField).toContain("align-items: center;");
    const switches = sourceSection(css, ".admin-model-editor-references .admin-capability-boolean-field label {", "@media (min-width: 601px)");
    expect(switches).toContain("display: flex;");
    expect(compactSource(css)).toContain(".admin-model-editor-references .admin-capability-reference-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: start;");
    expect(compactSource(css)).toContain(".admin-model-editor-modal .admin-capability-reference-grid { grid-template-columns: minmax(0, 1fr);");
});

test("model editor presents protocols in a searchable inline radio browser instead of a dropdown", async () => {
    const [source, css] = await Promise.all([Bun.file(new URL("../src/pages/admin/components/channel-model-editor.tsx", import.meta.url)).text(), Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text()]);
    const protocolSection = sourceSection(compactSource(source), '<Form.Item className="admin-model-protocol-field"', "{protocolError && (");

    expect(protocolSection).toContain("<ModelProtocolBrowser");
    expect(protocolSection).not.toContain("<Select");
    expect(compactSource(css)).toContain(".admin-model-protocol-field { grid-column: 1 / -1;");
});

test("channel model fetch requires explicit selection before import", async () => {
    const [componentSource, apiSource, adminCssSource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/components/channel-model-manager.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/services/api/wallet.ts", import.meta.url)).text(),
        Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text(),
    ]);
    const component = compactSource(componentSource);

    expect(apiSource).toContain("http.post<{ models: string[] }>(`/admin/channels/${encodeURIComponent(channelId)}/models/fetch`)");
    expect(apiSource).toContain("http.post<{ models: string[]; added: number }>(`/admin/channels/${encodeURIComponent(channelId)}/models/import`, { models })");
    expect(component).toContain('title="选择要导入的模型"');
    expect(component).toContain("默认已全选");
    expect(component).toContain("setFetchPreviewOpen(true)");
    expect(component).toContain("setSelectedFetchModels(result.models)");
    expect(component).toContain("已选择 {selectedFetchModels.length} / {fetchPreviewModels.length} 个模型");
    expect(component).toContain("disabled={importing || allFetchModelsSelected}");
    expect(component).toContain("onClick={() => setSelectedFetchModels(fetchPreviewModels)}");
    expect(component).toContain("disabled={importing || selectedFetchModels.length === 0}");
    expect(component).toContain("onClick={() => setSelectedFetchModels([])}");
    expect(component).toContain("取消全选");
    expect(component).toContain("disabled={importing}");
    expect(component).toContain("importAdminChannelModels(channel.id, selectedFetchModels)");
    expect(component).toContain("disabled={!selectedFetchModels.length}");
    expect(component).not.toContain("disabled: alreadyExists");
    expect(component).not.toContain("const result = await fetchAdminChannelModels(channel.id); await reload();");
    expect(adminCssSource).toContain(".admin-model-import-modal .channel-model-import-picker .ant-checkbox-checked");
    expect(adminCssSource).toContain("border-color: var(--control-check-fg) !important");
});

test("channel model manager supports bounded atomic batch deletion", async () => {
    const [componentSource, apiSource] = await Promise.all([Bun.file(new URL("../src/pages/admin/components/channel-model-manager.tsx", import.meta.url)).text(), Bun.file(new URL("../src/services/api/wallet.ts", import.meta.url)).text()]);
    const component = compactSource(componentSource);

    expect(apiSource).toContain("http.post<{ deleted: number }>(`/admin/channels/${encodeURIComponent(channelId)}/models/batch-delete`, { modelIds })");
    expect(component).toContain("<AdminBatchBar count={selectedModelIds.length}");
    expect(component).toContain("rowSelection:");
    expect(component).toContain("selectedRowKeys: selectedModelIds");
    expect(component).toContain("preserveSelectedRowKeys: true");
    expect(component).toContain("setSelectedModelIds(next.slice(0, 100))");
    expect(component).toContain("deleteAdminChannelModels(channel.id, selectedModelIds)");
    expect(component).toContain("okButtonProps: { danger: true }");
    expect(component).toContain("本次就不会删除任何模型");
    expect(component).toContain("批量删除");
});

test("analytics keeps range presets and uses order finances without a separate pricing editor", async () => {
    const source = compactSource(await Bun.file(new URL("../src/pages/admin/components/analytics-panel.tsx", import.meta.url)).text());

    expect(source).toContain('type RangePreset = "7d" | "30d" | "60d"');
    expect(source).toContain('["60d", "60 天"]');
    expect(source).toContain('next.set("rangePreset", rangePreset)');
    expect(source).toContain("setRangePreset(undefined)");
    expect(source).not.toContain("模型价格配置");
    expect(source).not.toContain("listAdminModelPricings");
    expect(source).toContain("finance.revenueMicrocredits");
    expect(source).toContain("finance.profitMicrocredits");
    expect(source).toContain("finance.costedOrders < finance.settledOrders");
    expect(source).toContain("...analyticsFinanceColumns");
    expect(source).toContain("后端未返回完整财务统计");
});

test("storage settings keep generic S3 controls and connection validation", async () => {
    const source = await Bun.file(new URL("../src/pages/admin/settings/storage-settings-page.tsx", import.meta.url)).text();
    const compacted = compactSource(source);

    expect(compacted).toContain('{ mode: "s3", label: "S3 兼容存储"');
    expect(compacted).toContain("testAdminOSSConnection(connectionInput(values))");
    for (const field of ["s3Preset", "sessionToken", "pathStyle", "allowUserS3"]) {
        expect(compacted).toContain(`name="${field}"`);
    }
    expect(compacted).toContain('["aliyun", "tencent", "qiniu", "s3"].includes(setting.provider || "")');
});

test("customer service settings keep comfortable card padding", async () => {
    const [source, css] = await Promise.all([Bun.file(new URL("../src/pages/admin/customer-service/customer-service-page.tsx", import.meta.url)).text(), Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text()]);

    expect(source).toContain("admin-customer-service-settings");
    expect(compactSource(css)).toContain("[data-admin-root] .admin-customer-service-settings .admin-settings-section-summary { padding: 18px 24px;");
    expect(compactSource(css)).toContain("[data-admin-root] .admin-customer-service-settings .admin-settings-section-content { padding: 22px 24px 24px;");
    expect(compactSource(css)).toContain("[data-admin-root] .admin-customer-service-settings .admin-settings-section-content { padding: 18px 16px 20px;");
});

test("top-up product editor uses a centered responsive modal", async () => {
    const [source, css] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/product-operations/product-operations-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/product-operations/product-operations-page.css", import.meta.url)).text(),
    ]);
    const productEditor = sourceSection(source, "<AdminModal\n                centered\n                title={productDrawer", "</AdminModal>");

    expect(productEditor).toContain('title={productDrawer ? "编辑充值商品" : "新增充值商品"}');
    expect(productEditor).toContain('rootClassName="admin-payment-product-modal"');
    expect(productEditor).toContain('width="min(1180px, calc(100vw - 32px))"');
    expect(productEditor).toContain("maskClosable={!productSaving}");
    expect(productEditor).toContain('className="admin-payment-product-preview"');
    expect(productEditor).toContain("<CreditProductCard");
    expect(source).toContain("Form.useWatch([], productForm)");
    expect(productEditor).toContain("保存商品");
    expect(productEditor).not.toContain("<Drawer");
    expect(compactSource(css)).toContain(".admin-payment-product-modal .ant-modal-body { max-height: min(72vh, 680px);");
    expect(compactSource(css)).toContain(".admin-payment-product-editor-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(320px, 370px);");
    expect(compactSource(css)).toContain(".admin-payment-product-modal .grid.grid-cols-2 { grid-template-columns: minmax(0, 1fr);");
});

test("commerce and finance are separate collapsible navigation groups", async () => {
    const [shell, router, payments, productOperations] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/components/admin-shell.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/router.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/payments/payments-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/product-operations/product-operations-page.tsx", import.meta.url)).text(),
    ]);

    const commerceGroup = sourceSection(shell, 'id: "commerce"', 'id: "finance"');
    const financeGroup = sourceSection(shell, 'id: "finance"', 'id: "content"');

    expect(commerceGroup).toContain('label: "商品运营"');
    expect(commerceGroup).toContain("collapsible: true");
    expect(commerceGroup).toContain('{ path: "/admin/product-operations", label: "商品管理"');
    expect(commerceGroup).toContain('{ path: "/admin/redemption-codes", label: "兑换码"');
    expect(financeGroup).toContain('label: "财务管理"');
    expect(financeGroup).toContain("collapsible: true");
    expect(financeGroup).toContain('{ path: "/admin/payments", label: "支付渠道"');
    expect(financeGroup).toContain('{ path: "/admin/payment-orders", label: "支付订单"');
    expect(financeGroup).toContain('{ path: "/admin/payment-reconciliation", label: "支付对账"');
    expect(financeGroup).toContain('{ path: "/admin/credit-operations", label: "积分运营"');
    expect(commerceGroup).not.toContain('path: "/admin/payments"');
    expect(commerceGroup).not.toContain('path: "/admin/payment-orders"');
    expect(commerceGroup).not.toContain('path: "/admin/payment-reconciliation"');
    expect(commerceGroup).not.toContain('path: "/admin/credit-operations"');
    expect(router).toContain('{ path: "product-operations", element: adminRoute("admin.commerce.products", <ProductOperationsPage />) }');
    expect(router).toContain('{ path: "payments", element: adminRoute("admin.finance.payment_providers", <AdminPaymentsPage view="providers" />) }');
    expect(router).toContain('{ path: "payment-orders", element: adminRoute("admin.finance.payment_orders", <AdminPaymentsPage view="orders" />) }');
    expect(router).toContain('{ path: "payment-reconciliation", element: adminRoute("admin.finance.reconciliation", <AdminPaymentsPage view="reconciliation" />) }');
    expect(productOperations).toContain('title="商品管理"');
    expect(productOperations).toContain('label: "商品管理"');
    expect(payments).not.toContain('label: "充值商品"');
    expect(payments).not.toContain("listAdminTopupProducts");
    expect(payments).not.toContain("<Tabs");
    expect(payments).toContain('providers: { title: "支付渠道"');
    expect(payments).toContain('orders: { title: "支付订单"');
    expect(payments).toContain('reconciliation: { title: "支付对账"');
});

test("admin navigation keeps the storage resource page reachable", async () => {
    const [source, chromeCss] = await Promise.all([Bun.file(new URL("../src/pages/admin/components/admin-shell.tsx", import.meta.url)).text(), Bun.file(new URL("../src/pages/admin/theme/admin-chrome.css", import.meta.url)).text()]);
    const storageGroup = sourceSection(source, 'id: "storage"', "\n];");
    expect(source).toContain('path: "/admin/resources"');
    expect(storageGroup).toContain('label: "存储与备份"');
    expect(storageGroup).toContain('path: "/admin/resources"');
    expect(storageGroup).toContain('path: "/admin/settings/runtime-policy"');
    expect(storageGroup).toContain('path: "/admin/settings/storage"');
    expect(source).toContain('className="admin-nav-group-toggle"');
    expect(source).toContain("aria-expanded={isOpen}");
    expect(source).toContain("inert={!isOpen}");
    expect(source).not.toContain('className="admin-sidebar-nav thin-scrollbar');
    expect(compactSource(chromeCss)).toContain(".admin-sidebar-nav::-webkit-scrollbar { display: none;");
});

test("business analytics keeps overview and request details while user tools stay separate", async () => {
    const source = await Bun.file(new URL("../src/pages/admin/components/admin-shell.tsx", import.meta.url)).text();
    const analyticsGroup = sourceSection(source, 'id: "analytics"', 'id: "platform"');

    expect(analyticsGroup).toContain('label: "经营分析"');
    expect(analyticsGroup).toContain("collapsible: true");
    expect(analyticsGroup).toContain('{ path: "/admin", label: "数据概览"');
    expect(analyticsGroup).toContain('{ path: "/admin/logs", label: "请求明细"');
    const platformGroup = sourceSection(source, 'id: "platform"', 'id: "users"');
    const usersGroup = sourceSection(source, 'id: "users"', 'id: "commerce"');
    expect(usersGroup).toContain('label: "用户与服务"');
    expect(usersGroup).toContain('{ path: "/admin/users", label: "用户管理"');
    expect(usersGroup).toContain('{ path: "/admin/administrators", label: "管理员配置"');
    expect(usersGroup).toContain("fullAdminOnly: true");
    expect(usersGroup).toContain('{ path: "/admin/customer-service", label: "客服配置"');
    expect(usersGroup).toContain('{ path: "/admin/agent-lessons", label: "Agent 记忆"');
    expect(analyticsGroup).not.toContain('path: "/admin/users"');
    expect(platformGroup).not.toContain('path: "/admin/users"');
    expect(source).not.toContain('id: "overview"');
    expect(source).not.toContain('label: "概览"');
});

test("ordinary users and administrators use separate guarded management flows", async () => {
    const [router, guard, api, usersPanel, administratorsPanel, createDrawer] = await Promise.all([
        Bun.file(new URL("../src/router.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/components/auth/require-admin-permission.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/services/api/auth.ts", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/users/users-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/users/administrators-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/users/administrator-create-drawer.tsx", import.meta.url)).text(),
    ]);

    expect(router).toContain('path: "administrators"');
    expect(router).toContain("<RequireFullAdmin><AdministratorsPage /></RequireFullAdmin>");
    expect(guard).toContain('user.adminAccess?.level === "full"');
    expect(api).toContain('http.get<{ users: AdminUser[]; total: number; page: number; pageSize: number }>("/admin/administrators"');
    expect(api).toContain('http.post<{ user: AdminManagedUser }>("/admin/administrators/promote"');
    expect(usersPanel).not.toContain("role: state.role");
    expect(usersPanel).not.toContain('label: "全部角色"');
    expect(administratorsPanel).toContain("listAdministrators");
    expect(administratorsPanel).toContain('accountKind="administrator"');
    expect(createDrawer).toContain('label: "创建新账号"');
    expect(createDrawer).toContain('label: "晋升现有用户"');
});

test("featured inspiration operations stay connected from admin to the creation workspace", async () => {
    const [shellSource, routeSource, panelSource, apiSource, workspaceSource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/components/admin-shell.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/router.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/admin-inspirations-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/services/api/inspirations.ts", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/create/creation-workspace.tsx", import.meta.url)).text(),
    ]);
    const contentGroup = sourceSection(shellSource, 'id: "content"', 'id: "settings"');

    expect(contentGroup).toContain('label: "内容与通知"');
    expect(contentGroup).toContain('path: "/admin/inspirations"');
    expect(contentGroup).toContain('label: "提示词运营"');
    expect(contentGroup).toContain('path: "/admin/announcements"');
    expect(contentGroup).toContain('path: "/admin/banner-announcements"');
    expect(routeSource).toContain('{ path: "inspirations", element: adminRoute("admin.content.inspirations", <InspirationsPage />) }');
    expect(panelSource).toContain("<AdminModal");
    expect(panelSource).toContain("centered");
    expect(panelSource).toContain('rootClassName="admin-inspiration-editor-modal"');
    expect(panelSource).not.toContain("<AdminDrawer");
    expect(panelSource).toContain("getAdminInspirationOrder");
    expect(panelSource).toMatch(/saveAdminInspirationOrder\(\s*orderItems\.map\(\(item\) => item\.id\),\s*orderOriginal,?\s*\)/);
    expect(panelSource).toContain('message.success(editing ? "精选灵感已更新" : "精选灵感已保存为停用状态")');
    expect(apiSource).toContain('http.put<{ saved: boolean }>("/admin/inspirations/order", { ids, expectedIds })');
    expect(workspaceSource).toContain("listInspirations(controller.signal)");
    expect(workspaceSource).toContain("creationInspirationDimensions(item)");
    expect(panelSource).toContain("readImageFileSize(file)");
    expect(apiSource).toContain('body.append("width", String(dimensions.width))');
    expect(workspaceSource).toContain('reason instanceof Error ? reason.message : `${source === "featured" ? "全部" : "个人"}灵感暂时无法加载`');
    expect(workspaceSource).not.toContain("creationFeaturedWorks");
});

test("nested admin pages return to their own parent entry", async () => {
    const source = await Bun.file(new URL("../src/pages/admin/components/admin-shell.tsx", import.meta.url)).text();
    const compacted = compactSource(source);
    expect(compacted).toContain("const currentItem = currentSection?.items.find");
    expect(compacted).toContain("const sectionPath = back ? (currentItem?.path");
});

test("all feature availability changes require confirmation before saving", async () => {
    const source = await Bun.file(new URL("../src/pages/admin/components/feature-availability-panel.tsx", import.meta.url)).text();
    expect(source).toContain('title: "确认开启短剧创作？"');
    expect(source).toContain('title: "确认关闭任务中心？"');
    expect(source).toContain('title: "确认开启积分计费？"');
    expect(source).toContain('title: "确认关闭自定义渠道？"');
    expect(source).toContain('title: "确认开启插件中心？"');
    expect(source).toContain('title: "确认隐藏系统插件？"');
    expect(source).toContain('title: "确认切换为前台模型目录？"');
    expect(source).toContain('title: "确认切换为系统渠道？"');
    expect(source).toContain('const copy = row.changeCopy[enabled ? "enabled" : "disabled"]');
    expect(source).toContain("<strong>前端用户影响：</strong>");
    expect(source).toContain("{copy.userImpact}");
    expect(source).toContain("onOk: () => setFeature(key, enabled)");
    expect(source).not.toContain("void setFeature(key, enabled)");
    expect(source).toContain("onChange={requestFeatureChange}");
});

test("admin settings use full-width summaries without selected-card side stripes", async () => {
    const [componentSource, cssSource] = await Promise.all([Bun.file(new URL("../src/pages/admin/components/admin-ui.tsx", import.meta.url)).text(), Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text()]);

    expect(componentSource).not.toContain("lg:grid lg:grid-cols-4");
    expect(cssSource).not.toContain('content: "配置摘要"');
    expect(cssSource).not.toContain("grid-template-columns: minmax(0, 1fr) 344px");
    expect(cssSource).not.toContain(".admin-storage-mode-choice::before");

    const featureSelected = sourceSection(cssSource, ".admin-feature-board-row.is-selected {", ".admin-feature-board-row.is-dirty {");
    const drawingSelected = sourceSection(cssSource, ".admin-drawing-engine-choice.is-selected {", ".admin-drawing-engine-choice.is-unavailable");
    expect(featureSelected).not.toContain("inset 3px 0 0");
    expect(drawingSelected).not.toContain("inset 3px 0 0");
});

test("task-first settings reveal dependent configuration only after the primary choice", async () => {
    const [storageSource, emailSource, accessSource, featureSource, appearanceSource, drawingSource, arkSource, interceptionSource, thirdPartySource, cssSource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/settings/storage-settings-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/email-settings-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/access-settings-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/feature-availability-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/settings/appearance-settings-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/settings/drawing-engine-settings-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/settings/ark-private-assets-settings-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/settings/response-interception-settings-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/settings/libtv-settings-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text(),
    ]);

    expect(storageSource).toContain('title="1. 选择新资源存储位置"');
    expect(storageSource).toContain("选择后继续完成第 2 步并保存");
    expect(sourceSection(storageSource, "const requestModeChange", "const save")).not.toContain("save(values)");

    expect(emailSource).toContain("{draftEnabled ? (");
    expect(emailSource).toContain('id="admin-email-smtp"');
    expect(emailSource).toContain('title="1. 是否发送账户安全邮件"');
    expect(emailSource).toContain('title="2. 配置 SMTP 连接与发件身份"');

    expect(accessSource).toContain("{draftLinuxDOEnabled ? (");
    expect(accessSource).toContain('title="1. 是否允许创建新账号"');
    expect(accessSource).toContain('title="2. 是否开放 Linux.do 登录"');

    expect(featureSource).toContain('title="1. 用户工作台入口"');
    expect(featureSource).toContain('title="2. 插件开放范围"');
    expect(featureSource).toContain('title="3. 用户模型来源"');
    expect(appearanceSource).not.toContain("<WelcomeSetting />");

    expect(drawingSource).toContain('title="1. 选择新建绘图默认编辑器"');
    expect(drawingSource).toContain('title="2. 配置 tldraw 授权（按需）"');
    expect(sourceSection(drawingSource, "const selectEngine", "async function save")).not.toContain("save(");

    expect(arkSource).toContain('title="1. 配置方舟项目与 IAM 凭据"');
    expect(arkSource).toContain('title="2. 是否启用可信素材同步"');
    expect(arkSource).toContain("{prerequisitesReady || draftEnabled ? (");
    expect(arkSource).toContain('aria-label="启用可信素材同步，保存修改后生效"');

    expect(interceptionSource).toContain('title="1. 是否替换用户可见的上游错误"');
    expect(interceptionSource).toContain('title="2. 配置替换规则与优先级"');
    expect(interceptionSource).toContain('title="3. 本地预览用户最终文案"');
    expect(interceptionSource).toContain("{enabled ? (");
    expect(interceptionSource).not.toContain('className="admin-intercept-overview"');
    expect(sourceSection(interceptionSource, "const changeEnabled", "if (loading")).not.toContain("save(");

    expect(thirdPartySource).toContain('title="1. 配置 LibTV 服务端访问凭据"');
    expect(thirdPartySource).toContain('title="2. 是否开放用户导入 LibTV 画布"');
    expect(thirdPartySource).toContain('title="3. 验证已保存的 LibTV 凭据"');
    expect(thirdPartySource).toContain("{draftHasToken ? (");
    expect(thirdPartySource).toContain("{setting.hasToken && !clearTokenDraft ? (");
    expect(thirdPartySource).not.toContain('className="admin-third-party-overview"');
    expect(sourceSection(thirdPartySource, "const changeEnabled", "const markTokenForRemoval")).not.toContain("save(");

    expect(compactSource(cssSource)).toContain(".admin-feature-board { width: 100%; max-width: none; grid-template-columns: minmax(0, 1fr);");
});

test("admin tables keep requested filters and actions in the intended positions", async () => {
    const [storageSource, creditSource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/components/storage-resources-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/credit-operations-panel.tsx", import.meta.url)).text(),
    ]);

    const storageToolbar = sourceSection(storageSource, "toolbar={", "toolbarActiveFilters=");
    expect(storageToolbar).toContain('className="admin-storage-resource-filters"');
    expect(storageToolbar).toContain('placeholder="资源 ID 或对象路径"');
    expect(storageToolbar).toContain('placeholder="搜索用户"');
    expect(storageToolbar).toContain("showSearch");
    expect(storageToolbar).toContain("filterOption={false}");
    expect(storageToolbar).toContain("onSearch={setUserSearch}");
    expect(storageSource).toContain("searchAdminUserReferences({ keyword: debouncedUserSearch || undefined, limit: 50 })");
    expect(storageToolbar).toContain('aria-label="筛选资源类型"');
    expect(storageToolbar).toContain('aria-label="筛选资源状态"');
    expect(storageToolbar).toContain('aria-label="筛选存储类型"');
    expect(storageSource).toContain("确认永久删除");
    expect(storageSource).toContain("用户头像、平台外观、客服资源及活动任务引用仍会受保护");
    expect(storageSource).toContain("previewAdminResourceDelete(uniqueIds)");
    expect(storageSource).toContain("删除首页灵感提示词展示图？");
    expect(storageSource).toContain("这是上传的提示词图展示图，确认要删除么？");
    expect(storageSource).toContain("deleteAdminResources(uniqueIds, hasInspirationCovers)");
    expect(storageSource).toContain("result.blocked.length > 0 || result.warnings.length > 0");
    expect(storageSource).toContain("<DeleteResultSummary blocked={result.blocked} warnings={result.warnings} />");

    const operationColumn = sourceSection(creditSource, 'title: "操作"', "const hasFilters");
    expect(operationColumn).toContain('fixed: "right"');
    expect(creditSource).toContain("<AdminModal");
    expect(creditSource).toContain('rootClassName="admin-credit-modal admin-credit-policy-modal"');
    expect(creditSource).toContain('rootClassName="admin-credit-modal admin-credit-adjustment-modal"');
    expect(creditSource).not.toContain("<Drawer");
});

test("request logs display user credit billing independently from upstream cost", async () => {
    const [listSource, detailSource, apiSource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/logs/logs-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/api-log-detail-drawer.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/services/api/auth.ts", import.meta.url)).text(),
    ]);

    const billingSummary = sourceSection(listSource, "function BillingSummary", "function MediaResult");
    expect(listSource).toContain('title: "积分计算"');
    expect(listSource).toContain('title: "请求阶段 / 状态"');
    expect(listSource).toContain('description="模型生成与结果下载记录；仅计费调用扣除积分"');
    expect(billingSummary).toContain("billingAmountMicrocredits");
    expect(billingSummary).toContain("billingAvailable");
    expect(billingSummary).toContain("!log.billable");
    expect(billingSummary).toContain("未扣积分");
    expect(billingSummary).not.toContain("costAvailable");
    expect(detailSource).toContain('["请求阶段", requestKindText(log.requestKind)]');
    expect(detailSource).toContain('["计费属性", log.billable ? "计费调用" : "不计费"]');
    expect(detailSource).toContain('["销售价格（积分）", billingText(log)]');
    expect(detailSource).toContain('["上游成本", log.costAvailable');
    expect(apiSource).toContain("billingAmountMicrocredits: number");
    expect(apiSource).toContain("billingAvailable: boolean");
});

test("banner announcement editor keeps title styles through edit, save and status toggle", async () => {
    const [panelSource, editorSource, sliderSource, apiSource, contentSource, emojiPickerSource, noticeSource] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/components/admin-banner-announcements-panel.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/banner-title-editor.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/components/layout/banner-announcements-slider.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/services/api/announcements.ts", import.meta.url)).text(),
        Bun.file(new URL("../src/components/layout/banner-announcement-content.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/banner-notice-emoji-picker.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/lib/announcements/banner-notice.ts", import.meta.url)).text(),
    ]);
    const panel = compactSource(panelSource);
    const slider = compactSource(sliderSource);

    // 状态开关与保存都必须带上样式分段与通知类型：后端更新走显式字段表，漏传会清空它们。
    expect(panel).toContain("titleRuns: banner.titleRuns");
    expect(panel).toContain("titleRuns: runs");
    expect(panel).toContain("noticeType: selectedNoticeType");
    expect(panel).toContain("noticeType: normalizeBannerNoticeType(banner.noticeType)");
    // 编辑入口不能被新建信号重置：父级 createOpen 消费后立即复位，编辑路径不再写回该标志。
    expect(panel).toContain("onCreateOpenChange(false)");
    expect(panel).toContain('setDialog({ mode: "edit", banner })');
    expect(panel).toContain("banner.titleRuns?.length");
    // AntD 6 用 destroyOnHidden，destroyOnClose 已废弃。
    expect(panel).toContain("destroyOnHidden");
    expect(panel).not.toContain("destroyOnClose");
    // Modal 渲染在 body portal 里，必须挂 admin-modal-root 才能拿到弹窗作用域的强边框 / 分层 token，
    // 否则编辑器等自绘控件的边框回落到 :root 的 8% 透明度，肉眼不可见。
    expect(panel).toContain('rootClassName="admin-modal-root"');
    // 非表单控件不能放进 Form.Item（会被注入 value/onChange/ref）。
    expect(panel).toContain("<BannerNoticePreview runs={titleRuns} hasLink={Boolean(linkValue?.trim())} noticeType={noticeType} />");
    // 通知类型在后台要有独立入口；emoji 图标不设独立字段，经编辑器「图标」按钮插入。
    expect(panel).toContain("<BannerNoticeTypeSelector");
    expect(panel).not.toContain("BannerNoticeIconPicker");
    expect(panel).not.toContain("normalizeBannerNoticeIcon");

    expect(editorSource).toContain("applyBannerTitleStyle");
    expect(editorSource).toContain("clearBannerTitleStyle");
    expect(editorSource).toContain("lowContrastBannerTitleColors");
    // 标题是单行语义，Enter 不产生新段落。
    expect(editorSource).toContain('event.key === "Enter"');
    // emoji 素材插到光标处、作为普通文本保存（无独立字段、无默认图标）。
    expect(editorSource).toContain("<BannerNoticeEmojiPopover");
    expect(editorSource).toContain("insertContent(emoji)");
    expect(editorSource).not.toContain("BannerAnnouncementIcon");
    // 预览必须走前台同一份展示单元，否则「预览即前台」不成立。
    expect(editorSource).toContain("bannerAnnouncementBarStyle(noticeType)");

    // 前台通知条必须按分段渲染，不能退回纯文本；底色来自当前通知的类型，标题前不再有固定图标。
    expect(sliderSource).toContain("BannerAnnouncementTitle");
    expect(sliderSource).toContain("currentBanner.titleRuns");
    expect(slider).toContain("bannerAnnouncementBarStyle(currentBanner.noticeType)");
    expect(slider).not.toContain("BannerAnnouncementIcon");
    expect(slider).not.toContain("notice-banner-surface");

    // 展示单元是唯一来源：底色、富文本标题、详情入口都在这里定义，前台与预览共用；
    // 图标素材是标题文本的一部分，展示单元不再单独处理图标。
    expect(contentSource).toContain("export function bannerAnnouncementBarStyle");
    expect(contentSource).toContain("export function BannerAnnouncementLinkHint");
    expect(contentSource).toContain("export function BannerAnnouncementTitle");
    expect(contentSource).not.toContain("BannerAnnouncementIcon");

    // emoji 面板：默认收起（Popover 点击触发），插入后不自动关闭，方便连续插入。
    expect(emojiPickerSource).toContain('trigger="click"');
    expect(emojiPickerSource).toContain("onPick(item.char)");
    expect(noticeSource).toContain("BANNER_NOTICE_EMOJI_GROUPS");
    expect(noticeSource).not.toContain("DEFAULT_ICON");

    expect(apiSource).toContain("titleRuns?: BannerTitleRun[]");
    expect(apiSource).toContain("noticeType?: BannerNoticeType");
    expect(apiSource).not.toContain("icon?:");
    expect(apiSource).toContain("export type { BannerTitleRun }");
});

test("runtime policy settings use a searchable category workspace with explicit draft controls", async () => {
    const [pageSource, cssSource] = await Promise.all([Bun.file(new URL("../src/pages/admin/settings/runtime-policy-settings-page.tsx", import.meta.url)).text(), Bun.file(new URL("../src/styles/admin-ui.css", import.meta.url)).text()]);

    expect(pageSource).toContain("const totalPolicyFieldCount = allPolicyFields.length");
    expect(pageSource).not.toContain("42 项");
    expect(pageSource).toContain('placeholder="搜索参数名称、说明或单位"');
    expect(pageSource).toContain("visibleFieldsBySection");
    expect(pageSource).toContain("changedOnly");
    expect(pageSource).toContain("<PolicyNumberControl");
    expect(pageSource).toContain('aria-label="撤销此项改动"');
    expect(pageSource).toContain("<Dropdown");
    expect(pageSource).not.toContain("AdminStatTile");
    expect(pageSource).not.toContain('className="admin-runtime-policy-overview"');

    expect(cssSource).toContain(".admin-runtime-policy-workspace-controls");
    expect(cssSource).toContain(".admin-runtime-policy-field.is-dirty");
    expect(compactSource(cssSource)).toContain(".admin-runtime-policy-field-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr));");
    expect(compactSource(cssSource)).toContain("@media (max-width: 1099px) { .admin-runtime-policy-field-grid { grid-template-columns: minmax(0, 1fr);");
});

test("admin console tokens and shell stay isolated from the user workspace", async () => {
    const [tokens, shell, chrome, globals] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/theme/admin-tokens.css", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/admin-shell.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/theme/admin-chrome.css", import.meta.url)).text(),
        Bun.file(new URL("../src/styles/globals.css", import.meta.url)).text(),
    ]);

    expect(tokens).toContain("--admin-canvas: #f7f8fa;");
    expect(tokens).toContain("--admin-canvas: #111317;");
    expect(tokens).not.toContain("--admin-layer-0: var(--workspace-");
    expect(tokens).not.toContain("--admin-layer-0: var(--skin-admin-");
    expect(shell).toContain("data-admin-root");
    expect(shell).toContain("getIsolatedAdminAntTheme");
    expect(shell).not.toContain("WorkspacePage");
    expect(shell).not.toContain("getAdminAntThemeConfig");
    expect(shell).not.toContain("app-workspace-nav-link");
    expect(chrome).toContain("[data-admin-root] .admin-nav-link");
    expect(chrome).toContain("border-left: 0 !important");
    expect(chrome).not.toContain("left: -8px");
    expect(chrome).toContain(".admin-drawer .ant-drawer-content");
    expect(globals).not.toContain("/* 管理端专用视觉收口：不覆盖创作端 workspace 的导航、状态和图表样式。 */");

    const [overlays, userDetail, prompts, payments, productOperations, modelEditor] = await Promise.all([
        Bun.file(new URL("../src/pages/admin/ui/overlays.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/admin-user-detail-drawer.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/storyboard-prompts/storyboard-prompts-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/payments/payments-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/product-operations/product-operations-page.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/pages/admin/components/channel-model-editor.tsx", import.meta.url)).text(),
    ]);
    expect(overlays).toContain('rootClassName={cn("admin-drawer"');
    expect(overlays).toContain('rootClassName={cn("admin-modal-root"');
    expect(overlays).not.toContain("@/components/ui/product");
    for (const source of [userDetail, prompts, payments, productOperations, modelEditor]) {
        expect(source).not.toContain("@/components/ui/product");
        expect(source).not.toContain("AppDrawer");
        expect(source).not.toContain("AppModal");
    }
});
