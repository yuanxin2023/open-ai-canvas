import { expect, test } from "bun:test";
import { resolveWorkbenchRestoreModel } from "@/pages/workbench/workbench-restore-model";

const options = [{ value: "channel-a::same" }, { value: "channel-b::same" }, { value: "managed::public", logicalModelId: "logical-original" }];
test("restoration uses the exact channel selector or public logical identity", () => {
    expect(resolveWorkbenchRestoreModel({ model: "same", modelSelector: "channel-b::same" }, options)).toBe("channel-b::same");
    expect(resolveWorkbenchRestoreModel({ model: "public", logicalModelId: "logical-original" }, options)).toBe("managed::public");
});
test("plain names and removed channels require an explicit choice", () => {
    expect(resolveWorkbenchRestoreModel({ model: "same" }, options)).toBeNull();
    expect(resolveWorkbenchRestoreModel({ modelSelector: "removed::same" }, options)).toBeNull();
    expect(resolveWorkbenchRestoreModel({ model: "channel-a::same" }, options)).toBeNull();
    expect(resolveWorkbenchRestoreModel({ model: "same" }, [{ value: "other-provider::same" }])).toBeNull();
});
test("ambiguous or missing logical identities never fall back to a same-name model", () => {
    expect(resolveWorkbenchRestoreModel({ logicalModelId: "logical-original" }, [...options, { value: "duplicate::public", logicalModelId: "logical-original" }])).toBeNull();
    expect(resolveWorkbenchRestoreModel({ logicalModelId: "removed", model: "channel-a::same" }, options)).toBeNull();
});
