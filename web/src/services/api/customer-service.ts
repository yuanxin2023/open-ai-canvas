import { http } from "@/services/api/request";

export type CustomerServicePosition = "bottom-right" | "bottom-left" | "top-right" | "top-left";
export type CustomerServiceDisplayType = "circle" | "pill" | "icon-text" | "custom-image";

export type PublicCustomerService = {
    schemaVersion: number;
    enabled: boolean;
    position: CustomerServicePosition;
    displayType: CustomerServiceDisplayType;
    color: string;
    label: string;
    imageUrl: string;
    imageConfigured: boolean;
    draggable: boolean;
    desktopEnabled: boolean;
    mobileEnabled: boolean;
    buttonSize: number;
    offsetX: number;
    offsetY: number;
    configured: boolean;
    revision: string;
    updatedAt?: string;
};

export type AdminCustomerService = Omit<PublicCustomerService, "imageUrl" | "imageConfigured" | "revision"> & {
    imageResourceId: string;
    public: PublicCustomerService;
    updatedBy?: string;
    createdAt?: string;
    updatedAt?: string;
};

export type CustomerServiceResource = {
    id: string;
    kind: string;
    status: string;
    mimeType: string;
    size: number;
};

export async function getPublicCustomerService(signal?: AbortSignal) {
    const result = await http.get<{ customerService: PublicCustomerService }>("/public/customer-service", { signal });
    return result.customerService;
}
export async function getAdminCustomerService(signal?: AbortSignal) {
    const result = await http.get<{ setting: AdminCustomerService }>("/admin/customer-service", { signal });
    return result.setting;
}

export async function updateAdminCustomerService(input: Pick<AdminCustomerService, "enabled" | "position" | "displayType" | "color" | "label" | "imageResourceId" | "draggable" | "desktopEnabled" | "mobileEnabled" | "buttonSize" | "offsetX" | "offsetY">) {
    const result = await http.patch<{ setting: AdminCustomerService }>("/admin/customer-service", input);
    return result.setting;
}

export async function uploadCustomerServiceButtonImage(file: File) {
    const body = new FormData();
    body.append("file", file);
    const result = await http.post<{ resource: CustomerServiceResource }>("/admin/customer-service/button-image", body);
    return result.resource;
}
