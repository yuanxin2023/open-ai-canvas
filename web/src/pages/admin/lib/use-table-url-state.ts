import { useCallback, useEffect, useMemo, useRef } from "react";
import { useSearchParams } from "react-router";

export type TableUrlState = {
    filter: string;
    status: string;
    page: number;
    pageSize: number;
};

function positiveInteger(value: string | null, fallback: number) {
    const parsed = Number(value);
    return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function allowedValue(value: string | null, allowed: string[], fallback: string) {
    return value && allowed.includes(value) ? value : fallback;
}

function readTableUrlState(searchParams: URLSearchParams, defaultPageSize: number): TableUrlState {
    return {
        filter: searchParams.get("filter") || "",
        status: allowedValue(searchParams.get("status"), ["all", "active", "disabled"], "all"),
        page: positiveInteger(searchParams.get("page"), 1),
        pageSize: [20, 50, 100].includes(positiveInteger(searchParams.get("pageSize"), defaultPageSize)) ? positiveInteger(searchParams.get("pageSize"), defaultPageSize) : defaultPageSize,
    };
}

export function useTableUrlState(defaultPageSize = 20) {
    const [searchParams, setSearchParams] = useSearchParams();
    const searchParamsRef = useRef(searchParams);
    const state = useMemo<TableUrlState>(() => readTableUrlState(searchParams, defaultPageSize), [defaultPageSize, searchParams]);

    useEffect(() => {
        searchParamsRef.current = searchParams;
    }, [searchParams]);

    useEffect(() => {
        if (!searchParams.has("role")) return;
        const next = new URLSearchParams(searchParams);
        next.delete("role");
        searchParamsRef.current = next;
        setSearchParams(next, { replace: true });
    }, [searchParams, setSearchParams]);

    const update = useCallback((patch: Partial<TableUrlState>, replace = false) => {
        setSearchParams((current) => {
            const next = new URLSearchParams(searchParamsRef.current || current);
            const merged = { ...readTableUrlState(next, defaultPageSize), ...patch };
            const defaults: TableUrlState = { filter: "", status: "all", page: 1, pageSize: defaultPageSize };
            (Object.keys(defaults) as Array<keyof TableUrlState>).forEach((key) => {
                const value = merged[key];
                if (value === defaults[key]) next.delete(key);
                else next.set(key, String(value));
            });
            searchParamsRef.current = next;
            return next;
        }, { replace });
    }, [defaultPageSize, setSearchParams]);

    return { state, update };
}
