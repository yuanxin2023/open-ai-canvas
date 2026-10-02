package model

type AdminLevel string
type AdminPermission string

const (
	AdminLevelFull   AdminLevel = "full"
	AdminLevelScoped AdminLevel = "scoped"
)

const (
	AdminPermissionAnalyticsOverview     AdminPermission = "admin.analytics.overview"
	AdminPermissionAPILogs               AdminPermission = "admin.analytics.api_logs"
	AdminPermissionChannels              AdminPermission = "admin.platform.channels"
	AdminPermissionLogicalModels         AdminPermission = "admin.platform.logical_models"
	AdminPermissionPlugins               AdminPermission = "admin.platform.plugins"
	AdminPermissionSkills                AdminPermission = "admin.platform.skills"
	AdminPermissionPromptTemplates       AdminPermission = "admin.platform.prompt_templates"
	AdminPermissionUsers                 AdminPermission = "admin.users.accounts"
	AdminPermissionCustomerService       AdminPermission = "admin.users.customer_service"
	AdminPermissionAgentLessons          AdminPermission = "admin.users.agent_lessons"
	AdminPermissionProducts              AdminPermission = "admin.commerce.products"
	AdminPermissionRedeemCodes           AdminPermission = "admin.commerce.redeem_codes"
	AdminPermissionPaymentProviders      AdminPermission = "admin.finance.payment_providers"
	AdminPermissionPaymentOrders         AdminPermission = "admin.finance.payment_orders"
	AdminPermissionPaymentReconciliation AdminPermission = "admin.finance.reconciliation"
	AdminPermissionCredits               AdminPermission = "admin.finance.credits"
	AdminPermissionInspirations          AdminPermission = "admin.content.inspirations"
	AdminPermissionAnnouncements         AdminPermission = "admin.content.announcements"
	AdminPermissionBannerAnnouncements   AdminPermission = "admin.content.banner_announcements"
	AdminPermissionAppearance            AdminPermission = "admin.settings.appearance"
	AdminPermissionFeatures              AdminPermission = "admin.settings.features"
	AdminPermissionDrawingEngine         AdminPermission = "admin.settings.drawing_engine"
	AdminPermissionSystemPerformance     AdminPermission = "admin.settings.system_performance"
	AdminPermissionAccess                AdminPermission = "admin.settings.access"
	AdminPermissionEmail                 AdminPermission = "admin.settings.email"
	AdminPermissionArkPrivateAssets      AdminPermission = "admin.settings.ark_private_assets"
	AdminPermissionResponseInterception  AdminPermission = "admin.settings.response_interception"
	AdminPermissionThirdParty            AdminPermission = "admin.settings.third_party"
	AdminPermissionSystemUpdate          AdminPermission = "admin.settings.system_update"
	AdminPermissionStorageResources      AdminPermission = "admin.storage.resources"
	AdminPermissionRuntimePolicy         AdminPermission = "admin.storage.runtime_policy"
	AdminPermissionStorageService        AdminPermission = "admin.storage.service"
)

var AllAdminPermissions = []AdminPermission{
	AdminPermissionAnalyticsOverview, AdminPermissionAPILogs,
	AdminPermissionChannels, AdminPermissionLogicalModels, AdminPermissionPlugins, AdminPermissionSkills, AdminPermissionPromptTemplates,
	AdminPermissionUsers, AdminPermissionCustomerService, AdminPermissionAgentLessons,
	AdminPermissionProducts, AdminPermissionRedeemCodes,
	AdminPermissionPaymentProviders, AdminPermissionPaymentOrders, AdminPermissionPaymentReconciliation, AdminPermissionCredits,
	AdminPermissionInspirations, AdminPermissionAnnouncements, AdminPermissionBannerAnnouncements,
	AdminPermissionAppearance, AdminPermissionFeatures, AdminPermissionDrawingEngine, AdminPermissionSystemPerformance,
	AdminPermissionAccess, AdminPermissionEmail, AdminPermissionArkPrivateAssets, AdminPermissionResponseInterception,
	AdminPermissionThirdParty, AdminPermissionSystemUpdate,
	AdminPermissionStorageResources, AdminPermissionRuntimePolicy, AdminPermissionStorageService,
}

func IsAdminPermission(value AdminPermission) bool {
	for _, permission := range AllAdminPermissions {
		if value == permission {
			return true
		}
	}
	return false
}

func (u *User) HasAdminPermission(permission AdminPermission) bool {
	if u == nil || u.Role != UserRoleAdmin {
		return false
	}
	if u.AdminLevel == AdminLevelFull {
		return true
	}
	for _, granted := range u.AdminPermissions {
		if granted == permission {
			return true
		}
	}
	return false
}
