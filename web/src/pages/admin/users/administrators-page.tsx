import { useAdminContext } from "../admin-context";
import { AdminPageFrame } from "../components/admin-shell";
import AdministratorsPanel from "./administrators-panel";

export default function AdministratorsPage() {
    const { updateUserReference, removeUserReference } = useAdminContext();
    return (
        <AdminPageFrame title="管理员配置" description="管理员账号、级别与模块权限">
            <AdministratorsPanel onUserChanged={updateUserReference} onUserDeleted={removeUserReference} />
        </AdminPageFrame>
    );
}
