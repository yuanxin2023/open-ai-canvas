import { forwardRef, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, ClipboardEvent, DragEvent, KeyboardEvent, MouseEvent, PointerEvent, TextareaHTMLAttributes } from "react";
import { createPortal } from "react-dom";
import { FileText, Globe2, Sparkles, UserRound } from "lucide-react";
import { observeMovablePopupAnchor } from "@/lib/movable-popup-anchor";

import { canvasThemes } from "@/lib/canvas-theme";
import { applySlashCommand, findSlashCommandMatch, type SlashCommandMatch } from "@/lib/canvas/slash-command";
import { useActiveTheme } from "@/stores/canvas/use-canvas-theme-store";
import { buildAssetMentionReferences, canvasResourceMentionToken, findCanvasResourceAutoLinkMatch, type CanvasResourceAutoLinkMatch, type CanvasResourceReference } from "@/lib/canvas/canvas-resource-references";
import { useAssetStore } from "@/stores/use-asset-store";
import { useResolvedCanvasResourceReferences } from "./use-resolved-canvas-resource-references";
import { InlineReferencePreview, referencePreviewUrl, syncInlineMentionPreviews } from "./canvas-mention-chips";
import { MentionMenu, clamp, mentionCaretRect, mentionMenuPosition } from "./canvas-mention-menu";
import { getEditableSelection, renderEditableContent, serializeEditableValue, setEditableSelection } from "./canvas-mention-editable";
import { mapCanvasTextOffset, mergeCanvasText } from "@/lib/canvas/canvas-text-merge";

export { TOOL_ICON_MAP } from "./canvas-mention-chips";

type MentionState = {
    start: number;
    end: number;
    query: string;
};

export type CanvasSlashCommandGroup = {
    id: string;
    label: string;
    emptyLabel?: string;
};

export type CanvasSlashCommandItem = {
    id: string;
    groupId: string;
    label: string;
    description?: string;
    badge?: string;
    value: string;
    searchText?: string;
    previewUrl?: string;
};

export type EditableSelection = {
    start: number;
    end: number;
};

export type MentionTextPart =
    | {
          type: "text";
          text: string;
      }
    | {
          type: "mention";
          token: string;
          reference: CanvasResourceReference;
      };

type Props = Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "onChange" | "value"> & {
    value: string;
    references: CanvasResourceReference[];
    onSelectReference?: (reference: CanvasResourceReference) => CanvasResourceReference | undefined;
    onChange: (value: string) => boolean | void;
    onSubmit?: () => void;
    containerClassName?: string;
    highlightLabels?: boolean;
    mentionMenuWidth?: number;
    sendOnEnter?: boolean | "both";
    onContentSizeChange?: (height: number) => void;
    includeAssetLibrary?: boolean;
    activeDropReferenceId?: string | null;
    onReferenceFilesDrop?: (reference: CanvasResourceReference, files: File[]) => void;
    autoLinkEnabled?: boolean;
    slashCommandGroups?: CanvasSlashCommandGroup[];
    slashCommandItems?: CanvasSlashCommandItem[];
    slashCommandLoading?: boolean;
    slashCommandError?: string;
    slashCommandMenuWidth?: number;
    onSlashCommandOpen?: () => void;
};

// 回车提交语义由调用方决定：false 只在 ⌘/Ctrl+Enter 提交，"both" 两种都提交；Shift+Enter 始终换行。
function shouldSubmitOnEnter(event: { key: string; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean }, sendOnEnter: boolean | "both") {
    if (event.key !== "Enter" || event.shiftKey) return false;
    const modifier = event.ctrlKey || event.metaKey;
    if (sendOnEnter === false) return modifier;
    if (sendOnEnter === "both") return true;
    return !modifier;
}

export const CanvasResourceMentionTextarea = forwardRef<HTMLTextAreaElement, Props>(function CanvasResourceMentionTextarea(
    { value, references, onSelectReference, onChange, onSubmit, onKeyDown, className, containerClassName, style, highlightLabels = true, mentionMenuWidth = 320, sendOnEnter = true, onContentSizeChange, includeAssetLibrary = false, activeDropReferenceId, onReferenceFilesDrop, autoLinkEnabled = false, slashCommandGroups = [], slashCommandItems = [], slashCommandLoading = false, slashCommandError, slashCommandMenuWidth = 400, onSlashCommandOpen, ...props },
    forwardedRef,
) {
    const rawTheme = useActiveTheme();
    const assets = useAssetStore((state) => state.assets);
    const theme = canvasThemes[rawTheme as keyof typeof canvasThemes] ?? canvasThemes.dark;
    const containerRef = useRef<HTMLDivElement | null>(null);
    const textareaRef = useRef<HTMLTextAreaElement | null>(null);
    const editorRef = useRef<HTMLDivElement | null>(null);
    const readOnly = Boolean(props.disabled || props.readOnly);
    const composingRef = useRef(false);
    const compositionBaseRef = useRef("");
    const [compositionValue, setCompositionValue] = useState<string | null>(null);
    const plainSelectionRef = useRef<{ value: string; start: number; end: number } | null>(null);
    const pendingSelectionRef = useRef<number | null>(null);
    const pendingScrollTopRef = useRef<number | null>(null);
    const lastRenderedValueRef = useRef("");
    const [mention, setMention] = useState<MentionState | null>(null);
    const [activeIndex, setActiveIndex] = useState(-1);
    const [slashCommand, setSlashCommand] = useState<SlashCommandMatch | null>(null);
    const [slashCommandGroupId, setSlashCommandGroupId] = useState("");
    const [slashCommandActiveIndex, setSlashCommandActiveIndex] = useState(-1);
    const slashCommandOpenRef = useRef(false);
    const [autoLinkCursor, setAutoLinkCursor] = useState<number | null>(null);
    const autoLinkSuggestionRef = useRef<HTMLButtonElement | null>(null);
    const [autoLinkPosition, setAutoLinkPosition] = useState<{ left: number; top: number } | null>(null);
    const [nativeDropReferenceId, setNativeDropReferenceId] = useState<string | null>(null);
    const [previewReference, setPreviewReference] = useState<CanvasResourceReference | null>(null);
    const canvasReferences = useResolvedCanvasResourceReferences(references);
    const rawAssetReferences = useMemo(() => includeAssetLibrary ? buildAssetMentionReferences(assets) : [], [assets, includeAssetLibrary]);
    const assetReferences = useResolvedCanvasResourceReferences(rawAssetReferences);
    const activeCanvasReferences = useMemo(() => canvasReferences.filter((item) => item.active), [canvasReferences]);
    // 工具标签只能经九宫格等面板入口插入，@ 引用菜单不再重复展示。
    const mentionCanvasReferences = useMemo(() => canvasReferences.filter((item) => item.kind !== "tool"), [canvasReferences]);
    const activeMentionCanvasReferences = useMemo(() => mentionCanvasReferences.filter((item) => item.active), [mentionCanvasReferences]);
    const availableReferences = useMemo(() => [...(onSelectReference ? mentionCanvasReferences : activeMentionCanvasReferences), ...assetReferences], [onSelectReference, mentionCanvasReferences, activeMentionCanvasReferences, assetReferences]);
    const candidates = useMemo(() => {
        if (!mention) return [];
        const query = mention.query.trim().toLowerCase();
        if (!query) return onSelectReference ? mentionCanvasReferences : activeMentionCanvasReferences;
        return availableReferences.filter((item) => `${item.label} ${item.title} ${item.kind} ${item.category || ""} ${item.text || ""}`.toLowerCase().includes(query));
    }, [onSelectReference, mentionCanvasReferences, activeMentionCanvasReferences, availableReferences, mention]);
    const activeSlashCommandGroupId = slashCommandGroups.some((group) => group.id === slashCommandGroupId)
        ? slashCommandGroupId
        : slashCommandGroups[0]?.id || "";
    const slashCommandCandidates = useMemo(() => {
        if (!slashCommand || !activeSlashCommandGroupId) return [];
        const query = slashCommand.query.trim().toLowerCase();
        return slashCommandItems.filter((item) => {
            if (item.groupId !== activeSlashCommandGroupId) return false;
            if (!query) return true;
            return (item.searchText || `${item.label} ${item.description || ""} ${item.badge || ""} ${item.value}`).toLowerCase().includes(query);
        });
    }, [activeSlashCommandGroupId, slashCommand, slashCommandItems]);
    const activeReferences = useMemo(() => {
        if (!highlightLabels) return [];
        return [...activeCanvasReferences, ...assetReferences.filter((item) => value.includes(canvasResourceMentionToken(item)))];
    }, [activeCanvasReferences, assetReferences, highlightLabels, value]);
    const useRichEditor = Boolean(activeReferences.length);
    const autoLinkMatch = useMemo<CanvasResourceAutoLinkMatch | null>(() => autoLinkEnabled && autoLinkCursor !== null ? findCanvasResourceAutoLinkMatch(value, autoLinkCursor, activeCanvasReferences) : null, [activeCanvasReferences, autoLinkCursor, autoLinkEnabled, value]);

    useEffect(() => {
        if (!slashCommandGroupId && slashCommandGroups[0]) setSlashCommandGroupId(slashCommandGroups[0].id);
    }, [slashCommandGroupId, slashCommandGroups]);

    useEffect(() => {
        setSlashCommandActiveIndex(-1);
    }, [activeSlashCommandGroupId, slashCommand?.query, slashCommandItems]);
    const reportContentSize = useCallback((element: HTMLElement | null) => {
        if (!element || !onContentSizeChange) return;
        const previous = { height: element.style.height, minHeight: element.style.minHeight, maxHeight: element.style.maxHeight, overflow: element.style.overflow };
        element.style.height = "1px";
        element.style.minHeight = "0";
        element.style.maxHeight = "none";
        element.style.overflow = "hidden";
        const height = Math.ceil(element.scrollHeight);
        element.style.height = previous.height;
        element.style.minHeight = previous.minHeight;
        element.style.maxHeight = previous.maxHeight;
        element.style.overflow = previous.overflow;
        onContentSizeChange(height);
    }, [onContentSizeChange]);

    useLayoutEffect(() => {
        if (!useRichEditor) pendingScrollTopRef.current = null;
    }, [useRichEditor, value]);

    useLayoutEffect(() => {
        if (!useRichEditor) return;
        const editor = editorRef.current;
        if (!editor || composingRef.current) return;
        const isFocused = document.activeElement === editor;
        const currentValue = serializeEditableValue(editor);
        if (currentValue === value && lastRenderedValueRef.current === value) {
            syncInlineMentionPreviews(editor, activeReferences);
            pendingSelectionRef.current = null;
            return;
        }
        const currentSelection = isFocused ? getEditableSelection(editor)?.start ?? null : null;
        const selection = pendingSelectionRef.current ?? (currentSelection === null ? null : mapCanvasTextOffset(currentValue, value, currentSelection));
        const scrollTop = pendingScrollTopRef.current;
        renderEditableContent(editor, value, activeReferences);
        lastRenderedValueRef.current = value;
        if (isFocused && selection !== null) setEditableSelection(editor, selection);
        pendingSelectionRef.current = null;
        reportContentSize(editor);
        if (scrollTop !== null) editor.scrollTop = scrollTop;
        pendingScrollTopRef.current = null;
    }, [activeReferences, reportContentSize, useRichEditor, value]);

    useLayoutEffect(() => {
        const editor = textareaRef.current, selection = plainSelectionRef.current;
        if (useRichEditor || composingRef.current || !editor || !selection || document.activeElement !== editor) return;
        const start = mapCanvasTextOffset(selection.value, value, selection.start), end = mapCanvasTextOffset(selection.value, value, selection.end);
        editor.setSelectionRange(start, end);
        plainSelectionRef.current = { value, start, end };
    }, [useRichEditor, value]);

    useLayoutEffect(() => {
        const anchor = useRichEditor ? editorRef.current : textareaRef.current;
        const suggestion = autoLinkSuggestionRef.current;
        if (!autoLinkMatch || !anchor || !suggestion) {
            setAutoLinkPosition(null);
            return;
        }
        const updatePosition = () => {
            const caret = mentionCaretRect(anchor, autoLinkMatch.end);
            const bounds = suggestion.getBoundingClientRect();
            setAutoLinkPosition({
                left: clamp(caret.right + 8, 12, window.innerWidth - bounds.width - 12),
                top: clamp(caret.bottom + 4, 12, window.innerHeight - bounds.height - 12),
            });
        };
        updatePosition();
        const observer = new ResizeObserver(updatePosition);
        observer.observe(anchor);
        observer.observe(suggestion);
        const stopMovingAnchor = observeMovablePopupAnchor(anchor, updatePosition);
        window.addEventListener("resize", updatePosition);
        window.addEventListener("scroll", updatePosition, true);
        return () => {
            observer.disconnect();
            stopMovingAnchor();
            window.removeEventListener("resize", updatePosition);
            window.removeEventListener("scroll", updatePosition, true);
        };
    }, [autoLinkMatch, useRichEditor, value]);

    useLayoutEffect(() => {
        const editor = editorRef.current;
        if (!editor) return;
        const dropReferenceId = nativeDropReferenceId || activeDropReferenceId || "";
        editor.querySelectorAll<HTMLElement>("[data-mention-reference-id]").forEach((chip) => {
            chip.classList.toggle("is-replace-target", Boolean(dropReferenceId) && chip.dataset.mentionReferenceId === dropReferenceId);
        });
    }, [activeDropReferenceId, nativeDropReferenceId, useRichEditor, value]);

    useLayoutEffect(() => {
        const element = useRichEditor ? editorRef.current : textareaRef.current;
        const container = containerRef.current;
        if (!element || !container || !onContentSizeChange) return;
        reportContentSize(element);
        let width = container.clientWidth;
        const observer = new ResizeObserver(() => {
            const nextWidth = container.clientWidth;
            if (nextWidth === width) return;
            width = nextWidth;
            reportContentSize(element);
        });
        observer.observe(container);
        return () => observer.disconnect();
    }, [onContentSizeChange, reportContentSize, useRichEditor, value]);

    const focusEditor = (selectionStart?: number) => {
        requestAnimationFrame(() => {
            const editor = editorRef.current;
            if (editor) {
                const scrollTop = editor.scrollTop;
                editor.focus({ preventScroll: true });
                if (typeof selectionStart === "number") setEditableSelection(editor, selectionStart);
                editor.scrollTop = scrollTop;
                return;
            }
            const textarea = textareaRef.current;
            if (!textarea) return;
            const scrollTop = textarea.scrollTop;
            textarea.focus({ preventScroll: true });
            if (typeof selectionStart === "number") textarea.setSelectionRange(selectionStart, selectionStart);
            textarea.scrollTop = scrollTop;
        });
    };

    const updateValue = (next: string, selectionStart?: number) => {
        if (readOnly || onChange(next) === false) return false;
        if (typeof selectionStart === "number") pendingSelectionRef.current = selectionStart;
        const editor = editorRef.current ?? textareaRef.current;
        pendingScrollTopRef.current = editor?.scrollTop ?? null;
        if (typeof selectionStart === "number") focusEditor(selectionStart);
        return true;
    };

    const closeMention = () => {
        setMention(null);
        setActiveIndex(-1);
    };

    const closeSlashCommand = () => {
        slashCommandOpenRef.current = false;
        setSlashCommand(null);
        setSlashCommandActiveIndex(-1);
    };

    const syncMention = (nextValue: string, cursor: number) => {
        if (readOnly) return;
        setAutoLinkCursor(cursor);
        const prefix = nextValue.slice(0, cursor);
        const match = /@([^\s@,.;:!?，。；：！？、)\]}】）]*)$/.exec(prefix);
        if (!match || !availableReferences.length) {
            closeMention();
            return;
        }
        const nextMention = { start: match.index, end: cursor, query: match[1] };
        const isSameMention = mention?.start === nextMention.start && mention.end === nextMention.end && mention.query === nextMention.query;
        if (!isSameMention) {
            setMention(nextMention);
            setActiveIndex(-1);
        }
    };

    const syncSlashCommand = (nextValue: string, cursor: number) => {
        if (!slashCommandGroups.length) {
            closeSlashCommand();
            return;
        }
        const nextSlashCommand = findSlashCommandMatch(nextValue, cursor);
        if (!nextSlashCommand) {
            closeSlashCommand();
            return;
        }
        closeMention();
        if (!slashCommandOpenRef.current) {
            slashCommandOpenRef.current = true;
            onSlashCommandOpen?.();
        }
        const isSameCommand = slashCommand?.start === nextSlashCommand.start
            && slashCommand.end === nextSlashCommand.end
            && slashCommand.query === nextSlashCommand.query;
        if (!isSameCommand) setSlashCommand(nextSlashCommand);
    };

    const syncMenus = (nextValue: string, cursor: number) => {
        syncMention(nextValue, cursor);
        syncSlashCommand(nextValue, cursor);
    };

    const insertReference = (reference: CanvasResourceReference) => {
        if (readOnly || !mention) return;
        const selected = onSelectReference ? onSelectReference(reference) : reference;
        if (!selected) return;
        const insertText = `${canvasResourceMentionToken(selected)} `;
        const next = `${value.slice(0, mention.start)}${insertText}${value.slice(mention.end)}`;
        closeMention();
        updateValue(next, mention.start + insertText.length);
    };

    const insertSlashCommand = (item: CanvasSlashCommandItem) => {
        if (!slashCommand) return;
        const next = applySlashCommand(value, slashCommand, item.value);
        closeSlashCommand();
        updateValue(next.value, next.cursor);
    };

    const replaceEditableSelection = (insertText: string) => {
        const currentValue = editorRef.current ? serializeEditableValue(editorRef.current) : value;
        const textarea = textareaRef.current;
        const selection = getEditableSelection(editorRef.current)
            || (textarea ? { start: textarea.selectionStart, end: textarea.selectionEnd } : null)
            || { start: currentValue.length, end: currentValue.length };
        const next = `${currentValue.slice(0, selection.start)}${insertText}${currentValue.slice(selection.end)}`;
        const cursor = selection.start + insertText.length;
        if (!updateValue(next, cursor)) return;
        syncMenus(next, cursor);
    };

    const insertAutoLink = (match: CanvasResourceAutoLinkMatch) => {
        const currentValue = editorRef.current ? serializeEditableValue(editorRef.current) : value;
        const insertText = `${canvasResourceMentionToken(match.reference)} `;
        const next = `${currentValue.slice(0, match.start)}${insertText}${currentValue.slice(match.end)}`;
        updateValue(next, match.start + insertText.length);
        setAutoLinkCursor(match.start + insertText.length);
    };

    const autoLinkSuggestion = !readOnly && autoLinkMatch ? createPortal(
        <button
            ref={autoLinkSuggestionRef}
            type="button"
            data-canvas-no-zoom
            className="fixed z-[var(--z-tooltip)] inline-flex max-w-[min(360px,calc(100vw-24px))] items-center gap-1.5 rounded-md border border-current/15 px-2 py-1 text-[var(--fs-micro)] shadow-sm"
            style={{ ...autoLinkPosition, visibility: autoLinkPosition ? "visible" : "hidden", background: theme.node.panel, color: theme.node.text }}
            onPointerDown={(event) => event.stopPropagation()}
            onMouseDown={(event) => { event.preventDefault(); event.stopPropagation(); }}
            onClick={(event) => { event.stopPropagation(); insertAutoLink(autoLinkMatch); }}
            aria-label={`引用${autoLinkMatch.reference.label}`}
        >
            <span className="truncate">引用「{autoLinkMatch.query}」→ @{autoLinkMatch.reference.label}</span>
            <kbd className="shrink-0 rounded border border-current/20 px-1 font-mono text-[var(--fs-micro)]">Tab</kbd>
        </button>,
        document.body,
    ) : null;

    const syncEditableValue = () => {
        if (readOnly || composingRef.current) return;
        const editor = editorRef.current;
        if (!editor) return;
        const next = serializeEditableValue(editor);
        const cursor = getEditableSelection(editor)?.start ?? next.length;
        if (onChange(next) === false) {
            renderEditableContent(editor, value, activeReferences);
            lastRenderedValueRef.current = value;
            setEditableSelection(editor, Math.min(cursor, value.length));
            return;
        }
        pendingSelectionRef.current = cursor;
        lastRenderedValueRef.current = next;
        syncMenus(next, cursor);
        reportContentSize(editor);
    };

    const syncEditableMenusFromSelection = () => {
        const editor = editorRef.current;
        if (!editor) return;
        const selection = getEditableSelection(editor);
        if (!selection || selection.start !== selection.end) {
            setAutoLinkCursor(null);
            return;
        }
        syncMenus(serializeEditableValue(editor), selection.start);
    };

    const referenceForDropTarget = (target: EventTarget | null) => {
        const element = target instanceof Element ? target.closest<HTMLElement>("[data-mention-reference-id]") : null;
        const referenceId = element?.dataset.mentionReferenceId;
        return referenceId ? activeReferences.find((reference) => reference.id === referenceId) : undefined;
    };

    const imageFilesFromTransfer = (event: DragEvent<HTMLDivElement>) => Array.from(event.dataTransfer.files).filter((file) => file.type.startsWith("image/"));

    const mergedStyle = {
        ...(style || {}),
        caretColor: style?.color || theme.node.text,
    } as CSSProperties;
    const menuAnchor = useRichEditor ? editorRef.current : textareaRef.current;
    const menu = !readOnly && mention && availableReferences.length && menuAnchor ? (
        <MentionMenu
            anchor={menuAnchor}
            connectedReferences={activeMentionCanvasReferences}
            assetReferences={assetReferences}
            filteredReferences={candidates}
            query={mention.query}
            cursorOffset={mention.end}
            activeReferenceId={activeIndex >= 0 ? candidates[Math.min(activeIndex, candidates.length - 1)]?.id : undefined}
            preferredWidth={mentionMenuWidth}
            onQueryChange={(query) => setMention((current) => current ? { ...current, query } : current)}
            onClose={closeMention}
            onSelect={insertReference}
        />
    ) : null;
    const slashMenu = slashCommand && slashCommandGroups.length && menuAnchor ? (
        <SlashCommandMenu
            anchor={menuAnchor}
            groups={slashCommandGroups}
            activeGroupId={activeSlashCommandGroupId}
            items={slashCommandCandidates}
            loading={slashCommandLoading}
            error={slashCommandError}
            query={slashCommand.query}
            cursorOffset={slashCommand.end}
            activeItemId={slashCommandActiveIndex >= 0 ? slashCommandCandidates[Math.min(slashCommandActiveIndex, slashCommandCandidates.length - 1)]?.id : undefined}
            preferredWidth={slashCommandMenuWidth}
            onGroupChange={setSlashCommandGroupId}
            onClose={closeSlashCommand}
            onSelect={insertSlashCommand}
        />
    ) : null;

    if (useRichEditor) {
        return (
            <div ref={containerRef} data-canvas-no-zoom className={`relative w-full min-h-0 overflow-hidden ${containerClassName || "h-full"}`}>
                {!value && props.placeholder ? (
                    <div aria-hidden className={`${className || ""} pointer-events-none absolute inset-0 z-0`} style={{ ...style, color: style?.color || theme.node.text, opacity: 0.4 }}>
                        {props.placeholder}
                    </div>
                ) : null}
                <div
                    ref={editorRef}
                    role="textbox"
                    aria-multiline="true"
                    aria-label={props["aria-label"]}
                    aria-disabled={props.disabled}
                    aria-readonly={readOnly}
                    contentEditable={!props.disabled && !props.readOnly}
                    suppressContentEditableWarning
                    spellCheck={props.spellCheck}
                    tabIndex={props.tabIndex}
                    className={`${className || ""} relative z-10 cursor-text select-text whitespace-pre-wrap break-words`}
                    style={{
                        ...mergedStyle,
                        color: style?.color || theme.node.text,
                        height: "100%",
                        minHeight: 0,
                        maxHeight: "100%",
                        overflowY: "auto",
                        overflowX: "hidden",
                    }}
                    onInput={syncEditableValue}
                    onCompositionStart={(event) => {
                        composingRef.current = true;
                        compositionBaseRef.current = serializeEditableValue(event.currentTarget);
                        props.onCompositionStart?.(event as unknown as React.CompositionEvent<HTMLTextAreaElement>);
                    }}
                    onCompositionEnd={(event) => {
                        const local = serializeEditableValue(event.currentTarget);
                        const cursor = getEditableSelection(event.currentTarget)?.start ?? local.length;
                        const merged = mergeCanvasText(compositionBaseRef.current, value, local);
                        composingRef.current = false;
                        renderEditableContent(event.currentTarget, merged, activeReferences);
                        setEditableSelection(event.currentTarget, mapCanvasTextOffset(local, merged, cursor));
                        syncEditableValue();
                        props.onCompositionEnd?.(event as unknown as React.CompositionEvent<HTMLTextAreaElement>);
                    }}
                    onPaste={(event: ClipboardEvent<HTMLDivElement>) => {
                        event.preventDefault();
                        replaceEditableSelection(event.clipboardData.getData("text/plain"));
                    }}
                    onDragOver={(event: DragEvent<HTMLDivElement>) => {
                        const reference = referenceForDropTarget(event.target);
                        const hasImageFile = Array.from(event.dataTransfer.items).some((item) => item.kind === "file" && (!item.type || item.type.startsWith("image/")))
                            || Array.from(event.dataTransfer.files).some((file) => file.type.startsWith("image/"));
                        if (!onReferenceFilesDrop || reference?.kind !== "image" || !hasImageFile) {
                            setNativeDropReferenceId(null);
                            props.onDragOver?.(event as unknown as React.DragEvent<HTMLTextAreaElement>);
                            return;
                        }
                        event.preventDefault();
                        event.dataTransfer.dropEffect = "copy";
                        setNativeDropReferenceId(reference.id);
                        props.onDragOver?.(event as unknown as React.DragEvent<HTMLTextAreaElement>);
                    }}
                    onDragLeave={(event: DragEvent<HTMLDivElement>) => {
                        const bounds = event.currentTarget.getBoundingClientRect();
                        if (event.clientX <= bounds.left || event.clientX >= bounds.right || event.clientY <= bounds.top || event.clientY >= bounds.bottom) setNativeDropReferenceId(null);
                        props.onDragLeave?.(event as unknown as React.DragEvent<HTMLTextAreaElement>);
                    }}
                    onDrop={(event: DragEvent<HTMLDivElement>) => {
                        if (readOnly) { event.preventDefault(); return; }
                        const reference = referenceForDropTarget(event.target);
                        const files = imageFilesFromTransfer(event);
                        setNativeDropReferenceId(null);
                        if (onReferenceFilesDrop && reference?.kind === "image" && files.length) {
                            event.preventDefault();
                            onReferenceFilesDrop(reference, files);
                        }
                        props.onDrop?.(event as unknown as React.DragEvent<HTMLTextAreaElement>);
                    }}
                    onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => {
                        if (readOnly) { onKeyDown?.(event as unknown as React.KeyboardEvent<HTMLTextAreaElement>); return; }
                        if (event.key === "Enter" && (event.nativeEvent.isComposing || composingRef.current || event.keyCode === 229)) return;
                        if (autoLinkMatch && event.key === "Tab" && !event.shiftKey && !event.nativeEvent.isComposing && !composingRef.current) {
                            event.preventDefault();
                            insertAutoLink(autoLinkMatch);
                            return;
                        }
                        if (slashCommand) {
                            if (event.key === "ArrowDown" && slashCommandCandidates.length) {
                                event.preventDefault();
                                setSlashCommandActiveIndex((index) => index < 0 ? 0 : (index + 1) % slashCommandCandidates.length);
                                return;
                            }
                            if (event.key === "ArrowUp" && slashCommandCandidates.length) {
                                event.preventDefault();
                                setSlashCommandActiveIndex((index) => index < 0 ? slashCommandCandidates.length - 1 : (index - 1 + slashCommandCandidates.length) % slashCommandCandidates.length);
                                return;
                            }
                            if (event.key === "Enter" || event.key === "Tab") {
                                event.preventDefault();
                                const item = slashCommandCandidates[slashCommandActiveIndex < 0 ? 0 : Math.min(slashCommandActiveIndex, slashCommandCandidates.length - 1)];
                                if (item) insertSlashCommand(item);
                                return;
                            }
                            if (event.key === "Escape") {
                                event.preventDefault();
                                closeSlashCommand();
                                return;
                            }
                        }
                        if (mention && candidates.length) {
                            if (event.key === "ArrowDown") {
                                event.preventDefault();
                                setActiveIndex((index) => index < 0 ? 0 : (index + 1) % candidates.length);
                                return;
                            }
                            if (event.key === "ArrowUp") {
                                event.preventDefault();
                                setActiveIndex((index) => index < 0 ? candidates.length - 1 : (index - 1 + candidates.length) % candidates.length);
                                return;
                            }
                            if (event.key === "Enter" || event.key === "Tab") {
                                event.preventDefault();
                                insertReference(candidates[activeIndex < 0 ? 0 : Math.min(activeIndex, candidates.length - 1)]);
                                return;
                            }
                            if (event.key === "Escape") {
                                event.preventDefault();
                                closeMention();
                                return;
                            }
                        }
                        if (event.key === "Enter") {
                            event.preventDefault();
                            const shouldSubmit = shouldSubmitOnEnter(event, sendOnEnter);
                            if (onSubmit && shouldSubmit) {
                                onSubmit();
                                return;
                            }
                            replaceEditableSelection("\n");
                            return;
                        }
                        onKeyDown?.(event as unknown as React.KeyboardEvent<HTMLTextAreaElement>);
                    }}
                    onKeyUp={(event) => {
                        if (event.key !== "Escape") syncEditableMenusFromSelection();
                        props.onKeyUp?.(event as unknown as React.KeyboardEvent<HTMLTextAreaElement>);
                    }}
                    onMouseDown={(event) => props.onMouseDown?.(event as unknown as React.MouseEvent<HTMLTextAreaElement>)}
                    onPointerDown={(event) => props.onPointerDown?.(event as unknown as React.PointerEvent<HTMLTextAreaElement>)}
                    onDoubleClick={(event) => {
                        const chip = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-mention-reference-id]") : null;
                        const reference = chip ? availableReferences.find((item) => item.id === chip.dataset.mentionReferenceId) : undefined;
                        if (!reference || !referencePreviewUrl(reference)) return;
                        event.preventDefault();
                        event.stopPropagation();
                        setPreviewReference(reference);
                    }}
                    onPointerUp={(event) => {
                        syncEditableMenusFromSelection();
                        props.onPointerUp?.(event as unknown as React.PointerEvent<HTMLTextAreaElement>);
                    }}
                    onSelect={(event) => props.onSelect?.(event as unknown as React.SyntheticEvent<HTMLTextAreaElement>)}
                    onWheel={(event) => {
                        event.stopPropagation();
                        props.onWheel?.(event as unknown as React.WheelEvent<HTMLTextAreaElement>);
                    }}
                    onScroll={(event) => props.onScroll?.(event as unknown as React.UIEvent<HTMLTextAreaElement>)}
                    onFocus={(event) => props.onFocus?.(event as unknown as React.FocusEvent<HTMLTextAreaElement>)}
                    onBlur={(event) => {
                        setAutoLinkCursor(null);
                        if (event.relatedTarget instanceof Element && event.relatedTarget.closest("[data-canvas-resource-mention-menu], [data-canvas-slash-command-menu]")) return;
                        window.setTimeout(() => {
                            if (document.activeElement?.closest("[data-canvas-resource-mention-menu], [data-canvas-slash-command-menu]")) return;
                            closeMention();
                            closeSlashCommand();
                        }, 120);
                        props.onBlur?.(event as unknown as React.FocusEvent<HTMLTextAreaElement>);
                    }}
                >
                </div>
                {autoLinkSuggestion}
                {menu}
                {slashMenu}
                {previewReference ? <InlineReferencePreview reference={previewReference} onClose={() => setPreviewReference(null)} /> : null}
            </div>
        );
    }

    return (
        <div ref={containerRef} data-canvas-no-zoom className={`relative w-full min-h-0 overflow-hidden ${containerClassName || "h-full"}`}>
            <textarea
                {...props}
                ref={(node) => {
                    textareaRef.current = node;
                    if (typeof forwardedRef === "function") forwardedRef(node);
                    else if (forwardedRef) forwardedRef.current = node;
                }}
                value={compositionValue ?? value}
                className={`${className || ""} relative z-10`}
                style={{ ...mergedStyle, height: "100%", minHeight: 0, maxHeight: "100%", overflowY: "auto", overflowX: "hidden" }}
                onChange={(event) => {
                    if (readOnly) return;
                    const next = event.target.value;
                    plainSelectionRef.current = { value: next, start: event.currentTarget.selectionStart, end: event.currentTarget.selectionEnd };
                    if (composingRef.current) { setCompositionValue(next); return; }
                    if (onChange(next) === false) { event.currentTarget.value = value; return; }
                    syncMenus(next, event.target.selectionStart);
                    reportContentSize(event.currentTarget);
                }}
                onCompositionStart={(event) => {
                    composingRef.current = true;
                    compositionBaseRef.current = event.currentTarget.value;
                    setCompositionValue(event.currentTarget.value);
                    props.onCompositionStart?.(event);
                }}
                onCompositionEnd={(event) => {
                    const local = event.currentTarget.value;
                    const merged = mergeCanvasText(compositionBaseRef.current, value, local);
                    const cursor = mapCanvasTextOffset(local, merged, event.currentTarget.selectionStart);
                    composingRef.current = false;
                    setCompositionValue(null);
                    event.currentTarget.value = merged;
                    event.currentTarget.setSelectionRange(cursor, cursor);
                    plainSelectionRef.current = { value: merged, start: cursor, end: cursor };
                    if (onChange(merged) === false) event.currentTarget.value = value;
                    syncMenus(merged, cursor);
                    props.onCompositionEnd?.(event);
                }}
                onSelect={(event) => {
                    const textarea = event.currentTarget;
                    plainSelectionRef.current = { value: textarea.value, start: textarea.selectionStart, end: textarea.selectionEnd };
                    const collapsedCursor = textarea.selectionStart === textarea.selectionEnd ? textarea.selectionStart : null;
                    setAutoLinkCursor(collapsedCursor);
                    if (collapsedCursor === null) {
                        closeMention();
                        closeSlashCommand();
                    } else {
                        syncMenus(textarea.value, collapsedCursor);
                    }
                    props.onSelect?.(event);
                }}
                onKeyDown={(event) => {
                    if (readOnly) { onKeyDown?.(event); return; }
if (event.key === "Enter" && (event.nativeEvent.isComposing || composingRef.current || event.keyCode === 229)) return;
                        if (autoLinkMatch && event.key === "Tab" && !event.shiftKey && !event.nativeEvent.isComposing && !composingRef.current) {
                            event.preventDefault();
                            insertAutoLink(autoLinkMatch);
                            return;
                        }
                    if (slashCommand) {
                        if (event.key === "ArrowDown" && slashCommandCandidates.length) {
                            event.preventDefault();
                            setSlashCommandActiveIndex((index) => index < 0 ? 0 : (index + 1) % slashCommandCandidates.length);
                            return;
                        }
                        if (event.key === "ArrowUp" && slashCommandCandidates.length) {
                            event.preventDefault();
                            setSlashCommandActiveIndex((index) => index < 0 ? slashCommandCandidates.length - 1 : (index - 1 + slashCommandCandidates.length) % slashCommandCandidates.length);
                            return;
                        }
                        if (event.key === "Enter" || event.key === "Tab") {
                            event.preventDefault();
                            const item = slashCommandCandidates[slashCommandActiveIndex < 0 ? 0 : Math.min(slashCommandActiveIndex, slashCommandCandidates.length - 1)];
                            if (item) insertSlashCommand(item);
                            return;
                        }
                        if (event.key === "Escape") {
                            event.preventDefault();
                            closeSlashCommand();
                            return;
                        }
                    }
                    if (mention && candidates.length) {
                        if (event.key === "ArrowDown") {
                            event.preventDefault();
                            setActiveIndex((index) => index < 0 ? 0 : (index + 1) % candidates.length);
                            return;
                        }
                        if (event.key === "ArrowUp") {
                            event.preventDefault();
                            setActiveIndex((index) => index < 0 ? candidates.length - 1 : (index - 1 + candidates.length) % candidates.length);
                            return;
                        }
                        if (event.key === "Enter") {
                            event.preventDefault();
                            insertReference(candidates[activeIndex < 0 ? 0 : Math.min(activeIndex, candidates.length - 1)]);
                            return;
                        }
                        if (event.key === "Escape") {
                            event.preventDefault();
                            closeMention();
                            return;
                        }
                    }
                    const shouldSubmit = shouldSubmitOnEnter(event, sendOnEnter);
                    if (shouldSubmit && onSubmit) {
                        event.preventDefault();
                        onSubmit();
                        return;
                    }
                    onKeyDown?.(event);
                }}
                onWheel={(event) => {
                    event.stopPropagation();
                    const textarea = event.currentTarget;
                    const deltaY = event.deltaMode === 1 ? event.deltaY * 16 : event.deltaMode === 2 ? event.deltaY * textarea.clientHeight : event.deltaY;
                    if (deltaY) {
                        const previousTop = textarea.scrollTop;
                        textarea.scrollTop += deltaY;
                        if (textarea.scrollTop !== previousTop) event.preventDefault();
                    }
                    props.onWheel?.(event);
                }}
                onBlur={(event) => {
                    setAutoLinkCursor(null);
                    if (event.relatedTarget instanceof Element && event.relatedTarget.closest("[data-canvas-resource-mention-menu], [data-canvas-slash-command-menu]")) return;
                    window.setTimeout(() => {
                        if (document.activeElement?.closest("[data-canvas-resource-mention-menu], [data-canvas-slash-command-menu]")) return;
                        closeMention();
                        closeSlashCommand();
                    }, 120);
                    props.onBlur?.(event);
                }}
            />
            {autoLinkSuggestion}
            {menu}
            {slashMenu}
        </div>
    );
});

function SlashCommandMenu({ anchor, groups, activeGroupId, items, loading, error, query, cursorOffset, activeItemId, preferredWidth, onGroupChange, onClose, onSelect }: {
    anchor: HTMLElement;
    groups: CanvasSlashCommandGroup[];
    activeGroupId: string;
    items: CanvasSlashCommandItem[];
    loading: boolean;
    error?: string;
    query: string;
    cursorOffset: number;
    activeItemId?: string;
    preferredWidth: number;
    onGroupChange: (groupId: string) => void;
    onClose: () => void;
    onSelect: (item: CanvasSlashCommandItem) => void;
}) {
    const menuRef = useRef<HTMLDivElement | null>(null);
    const scrollRef = useRef<HTMLDivElement | null>(null);
    const itemRefs = useRef(new Map<string, HTMLButtonElement>());
    const selectedRef = useRef(false);
    const [position, setPosition] = useState(() => mentionMenuPosition(anchor, cursorOffset, preferredWidth));
    const [hoveredItemId, setHoveredItemId] = useState<string | null>(null);
    const [failedPreviewId, setFailedPreviewId] = useState<string | null>(null);
    const [previewPosition, setPreviewPosition] = useState<{ left: number; top: number; side: "left" | "right" } | null>(null);
    const activeGroup = groups.find((group) => group.id === activeGroupId) || groups[0];
    const previewItem = items.find((item) => item.id === (hoveredItemId || activeItemId));
    const showPreview = Boolean(previewItem?.previewUrl && previewItem.id !== failedPreviewId);

    useLayoutEffect(() => {
        let frame = 0;
        const updatePosition = () => {
            cancelAnimationFrame(frame);
            frame = requestAnimationFrame(() => setPosition(mentionMenuPosition(anchor, cursorOffset, preferredWidth)));
        };
        updatePosition();
        const observer = new ResizeObserver(updatePosition);
        observer.observe(anchor);
        const stopMovingAnchor = observeMovablePopupAnchor(anchor, updatePosition);
        window.addEventListener("resize", updatePosition);
        window.addEventListener("scroll", updatePosition, true);
        return () => {
            cancelAnimationFrame(frame);
            observer.disconnect();
            stopMovingAnchor();
            window.removeEventListener("resize", updatePosition);
            window.removeEventListener("scroll", updatePosition, true);
        };
    }, [anchor, cursorOffset, preferredWidth]);

    useLayoutEffect(() => {
        if (!showPreview || !previewItem) {
            setPreviewPosition(null);
            return;
        }
        const updatePreviewPosition = () => {
            const menu = menuRef.current;
            const item = itemRefs.current.get(previewItem.id);
            if (!menu || !item) return;
            const menuBounds = menu.getBoundingClientRect();
            const itemBounds = item.getBoundingClientRect();
            const previewWidth = 288;
            const previewHeight = 264;
            const viewportInset = 12;
            const gap = 12;
            const roomRight = window.innerWidth - menuBounds.right;
            const roomLeft = menuBounds.left;
            const side = roomRight >= previewWidth + gap || roomRight >= roomLeft ? "right" : "left";
            const desiredLeft = side === "right" ? menuBounds.right + gap : menuBounds.left - previewWidth - gap;
            setPreviewPosition({
                left: clamp(desiredLeft, viewportInset, window.innerWidth - previewWidth - viewportInset),
                top: clamp(itemBounds.top, viewportInset, window.innerHeight - previewHeight - viewportInset),
                side,
            });
        };
        updatePreviewPosition();
        const menu = menuRef.current;
        const scroll = scrollRef.current;
        const observer = new ResizeObserver(updatePreviewPosition);
        if (menu) observer.observe(menu);
        const stopMovingAnchor = observeMovablePopupAnchor(anchor, updatePreviewPosition);
        scroll?.addEventListener("scroll", updatePreviewPosition, { passive: true });
        window.addEventListener("resize", updatePreviewPosition);
        window.addEventListener("scroll", updatePreviewPosition, true);
        return () => {
            observer.disconnect();
            stopMovingAnchor();
            scroll?.removeEventListener("scroll", updatePreviewPosition);
            window.removeEventListener("resize", updatePreviewPosition);
            window.removeEventListener("scroll", updatePreviewPosition, true);
        };
    }, [anchor, previewItem, showPreview]);

    useEffect(() => {
        setHoveredItemId(null);
        setFailedPreviewId(null);
    }, [activeGroupId, query]);

    useLayoutEffect(() => {
        const closeOnOutsidePointer = (event: globalThis.PointerEvent) => {
            const target = event.target;
            if (!(target instanceof Node) || menuRef.current?.contains(target) || anchor.contains(target)) return;
            onClose();
        };
        window.addEventListener("pointerdown", closeOnOutsidePointer, true);
        return () => window.removeEventListener("pointerdown", closeOnOutsidePointer, true);
    }, [anchor, onClose]);

    const selectItem = (item: CanvasSlashCommandItem) => {
        if (selectedRef.current) return;
        selectedRef.current = true;
        onSelect(item);
    };

    return createPortal(
        <>
        <div
            ref={menuRef}
            data-canvas-slash-command-menu="true"
            className="canvas-slash-command-menu fixed z-[var(--z-tooltip)]"
            data-placement={position.showAbove ? "top" : "bottom"}
            style={{ left: position.left, top: position.top, width: position.width, maxHeight: position.maxHeight, transform: position.showAbove ? "translateY(-100%)" : undefined }}
            onPointerDown={(event) => event.stopPropagation()}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => event.stopPropagation()}
            role="dialog"
            aria-label="调用提示词"
        >
            <header className="canvas-slash-command-header">
                <span><Sparkles aria-hidden="true" />提示词</span>
                <small>输入 /关键词 搜索</small>
            </header>
            <div className="canvas-slash-command-tabs" role="tablist" aria-label="提示词来源">
                {groups.map((group) => (
                    <button
                        key={group.id}
                        type="button"
                        role="tab"
                        aria-selected={group.id === activeGroupId}
                        className={group.id === activeGroupId ? "is-active" : undefined}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => onGroupChange(group.id)}
                    >
                        {group.id === "personal" ? <UserRound aria-hidden="true" /> : <Globe2 aria-hidden="true" />}
                        {group.label}
                    </button>
                ))}
            </div>
            <div ref={scrollRef} className="canvas-slash-command-scroll thin-scrollbar" role="listbox" aria-label={activeGroup?.label || "提示词"}>
                {loading ? <div className="canvas-slash-command-status"><span className="canvas-slash-command-spinner" aria-hidden="true" />正在加载提示词…</div>
                    : error ? <div className="canvas-slash-command-status is-error">{error}</div>
                        : items.length ? items.map((item) => (
                            <button
                                key={`${item.groupId}:${item.id}`}
                                ref={(node) => {
                                    if (node) itemRefs.current.set(item.id, node);
                                    else itemRefs.current.delete(item.id);
                                }}
                                type="button"
                                role="option"
                                aria-selected={item.id === activeItemId}
                                className={item.id === activeItemId ? "canvas-slash-command-item is-active" : "canvas-slash-command-item"}
                                onMouseEnter={() => setHoveredItemId(item.id)}
                                onMouseLeave={() => setHoveredItemId((current) => current === item.id ? null : current)}
                                onFocus={() => setHoveredItemId(item.id)}
                                onBlur={() => setHoveredItemId((current) => current === item.id ? null : current)}
                                onPointerDown={(event) => {
                                    event.preventDefault();
                                    event.stopPropagation();
                                    selectItem(item);
                                }}
                                onClick={(event) => {
                                    event.preventDefault();
                                    event.stopPropagation();
                                    selectItem(item);
                                }}
                            >
                                <span className="canvas-slash-command-icon"><FileText aria-hidden="true" /></span>
                                <span className="canvas-slash-command-copy">
                                    <span><strong>{item.label}</strong>{item.badge ? <em>{item.badge}</em> : null}</span>
                                    <small>{item.description || item.value}</small>
                                </span>
                            </button>
                        )) : <div className="canvas-slash-command-status">{query ? `没有匹配“${query}”的提示词` : activeGroup?.emptyLabel || "暂无提示词"}</div>}
            </div>
            <footer className="canvas-slash-command-footer"><kbd>↑↓</kbd> 选择 <kbd>Enter</kbd> 插入 <kbd>Esc</kbd> 关闭</footer>
        </div>
        {showPreview && previewItem && previewPosition ? (
            <aside
                className="canvas-slash-command-preview"
                data-side={previewPosition.side}
                style={{ left: previewPosition.left, top: previewPosition.top }}
                aria-hidden="true"
            >
                <img
                    src={previewItem.previewUrl}
                    alt=""
                    loading="lazy"
                    decoding="async"
                    referrerPolicy="no-referrer"
                    onError={() => setFailedPreviewId(previewItem.id)}
                />
                <span><strong>{previewItem.label}</strong>{previewItem.badge ? <em>{previewItem.badge}</em> : null}</span>
            </aside>
        ) : null}
        </>,
        anchor.closest(".ant-modal") || document.body,
    );
}
