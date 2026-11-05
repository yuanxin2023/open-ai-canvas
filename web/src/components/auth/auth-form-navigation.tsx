import { createContext, useCallback, useContext } from "react";
import { Link, useNavigate, useSearchParams, type LinkProps, type NavigateOptions } from "react-router";

export type AuthView = "login" | "register" | "forgot-password";
export const AuthFormContext = createContext<{
    next: string;
    navigate: (to: string) => void;
} | null>(null);

export function useAuthFormNavigation() {
    const modal = useContext(AuthFormContext);
    const routerNavigate = useNavigate();
    const [params] = useSearchParams();
    const value = modal?.next ?? params.get("next");
    const next = value?.startsWith("/") && !value.startsWith("//") ? value : "/";
    const navigate = useCallback(
        (to: string, options?: NavigateOptions) => {
            if (modal) modal.navigate(to);
            else void routerNavigate(to, options);
        },
        [modal, routerNavigate],
    );
    return { next, navigate };
}

export function AuthLink({ onClick, ...props }: LinkProps) {
    const modal = useContext(AuthFormContext);
    return (
        <Link
            {...props}
            onClick={(event) => {
                onClick?.(event);
                if (modal && !event.defaultPrevented) {
                    event.preventDefault();
                    modal.navigate(String(props.to));
                }
            }}
        />
    );
}
