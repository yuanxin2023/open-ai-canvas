import { AdminPageFrame } from "../components/admin-shell";
import { useAdminContext } from "../admin-context";
import UsersPanel from "./users-panel";

export default function UsersPage() {
    const { updateUserReference, removeUserReference } = useAdminContext();
    return (
        <AdminPageFrame title="用户管理" description="普通用户账号、积分与状态">
            <UsersPanel onUserChanged={updateUserReference} onUserDeleted={removeUserReference} />
        </AdminPageFrame>
    );
}
