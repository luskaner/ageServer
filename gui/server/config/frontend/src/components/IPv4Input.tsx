import React, { useMemo, useRef } from 'react';

export interface IPv4InputProps {
    value: string;
    onChange: (value: string) => void;
    className?: string;
    disabled?: boolean;
    id?: string;
    name?: string;
}

const emptySegments = ['', '', '', ''];

function parseSegments(value: string): string[] {
    if (!value) return [...emptySegments];

    if (value.includes('.')) {
        return value.split('.').slice(0, 4).concat(emptySegments).slice(0, 4);
    }

    const segments: string[] = [];
    for (let index = 0; index < value.length && segments.length < 4; index += 3) {
        segments.push(value.slice(index, index + 3));
    }
    return segments.concat(emptySegments).slice(0, 4);
}

function serializeSegments(segments: string[]): string {
    const lastFilledSegment = segments.reduce(
        (lastIndex, segment, index) => (segment ? index : lastIndex),
        -1,
    );
    return lastFilledSegment === -1 ? '' : segments.slice(0, lastFilledSegment + 1).join('.');
}

/** Keeps an octet as at most three digits and never above 255. */
function normalizeOctet(value: string): string {
    const digits = value.replace(/\D/g, '').slice(0, 3);
    return digits !== '' && Number(digits) > 255 ? '255' : digits;
}

function parsePastedValue(value: string): string[] | undefined {
    if (!/^[\d.]+$/.test(value)) return undefined;

    if (value.includes('.')) {
        const segments = value.split('.');
        return segments.length <= 4 ? segments.map(normalizeOctet) : undefined;
    }

    const segments: string[] = [];
    for (let index = 0; index < value.length; index += 3) {
        segments.push(value.slice(index, index + 3));
    }
    return segments.length <= 4 ? segments.map(normalizeOctet) : undefined;
}

/**
 * The next (or previous) element that Tab would reach, skipping the octets in
 * `container`: the address has to behave as a single form field, not four.
 */
function findFocusableOutside(container: HTMLElement, direction: 1 | -1): HTMLElement | undefined {
    const candidates = Array.from(
        document.querySelectorAll<HTMLElement>('a[href], button, input, select, textarea, [tabindex]'),
    ).filter((element) => {
        if (container.contains(element)) return false;
        if (element.tabIndex < 0) return false;
        if (element.matches(':disabled')) return false;
        if (element.getAttribute('aria-hidden') === 'true') return false;

        const style = window.getComputedStyle(element);
        if (style.display === 'none' || style.visibility === 'hidden') return false;

        const rect = element.getBoundingClientRect();
        return rect.width > 0 || rect.height > 0;
    });

    if (direction > 0) {
        return candidates.find(
            (element) => container.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_FOLLOWING,
        );
    }

    return candidates
        .reverse()
        .find((element) => container.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_PRECEDING);
}

/**
 * Renders an IPv4 address as four numeric fields that edit as one control, the
 * way the Windows address field does: the arrows cross the octets, Tab leaves
 * the whole address, Up/Down step the current octet and values above 255 are
 * clamped. Styled to match the Mantine inputs (see `.ipv4-input` in style.css).
 */
export const IPv4Input: React.FC<IPv4InputProps> = ({
    value,
    onChange,
    className = '',
    disabled = false,
    id,
    name,
}) => {
    const wrapperRef = useRef<HTMLDivElement | null>(null);
    const inputRefs = useRef<Array<HTMLInputElement | null>>([]);
    // Octet the caret was moved to by filling the previous one, so the dot that
    // follows a full octet does not skip a field (see handleSegmentChange).
    const autoAdvancedTo = useRef<number | null>(null);
    const segments = useMemo(() => parseSegments(value), [value]);
    const inputClassName = ['ipv4-input', className].filter(Boolean).join(' ');

    const focusSegment = (index: number, caret: 'start' | 'end' = 'start', select = false) => {
        const input = inputRefs.current[index];
        if (!input) return;

        input.focus();
        if (select && input.value) {
            input.select();
        } else {
            const position = caret === 'end' ? input.value.length : 0;
            input.setSelectionRange(position, position);
        }
    };

    const replaceSegment = (index: number, text: string) => {
        const nextSegments = [...segments];
        nextSegments[index] = text;
        onChange(serializeSegments(nextSegments));
    };

    const handleSegmentChange = (index: number, nextValue: string) => {
        const digits = normalizeOctet(nextValue);
        replaceSegment(index, digits);

        // Filling an octet moves on to the next one, with its content selected
        // so typing replaces it. The dot that usually follows a full octet is
        // then let through as the one that moved (see the '.' key below).
        if (digits.length === 3 && segments[index].length < 3 && index < 3) {
            autoAdvancedTo.current = index + 1;
            focusSegment(index + 1, 'start', true);
        }
    };

    const handlePaste = (event: React.ClipboardEvent<HTMLInputElement>, index: number) => {
        const pastedValue = event.clipboardData.getData('text').trim();
        const pastedSegments = parsePastedValue(pastedValue);
        if (!pastedSegments) return;

        event.preventDefault();
        const targetStart = pastedSegments.length === 4 ? 0 : index;
        const nextSegments = targetStart === 0 ? [...emptySegments] : [...segments];
        pastedSegments.forEach((segment, pastedIndex) => {
            const targetIndex = targetStart + pastedIndex;
            if (targetIndex < 4) nextSegments[targetIndex] = segment;
        });
        onChange(serializeSegments(nextSegments));
        focusSegment(Math.min(targetStart + pastedSegments.length - 1, 3), 'end');
    };

    const handleCopy = (event: React.ClipboardEvent<HTMLDivElement>) => {
        event.preventDefault();
        event.clipboardData.setData('text/plain', serializeSegments(segments));
    };

    const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>, index: number) => {
        const input = event.currentTarget;
        const start = input.selectionStart ?? 0;
        const end = input.selectionEnd ?? 0;

        // Only the dot typed straight after the automatic move is swallowed.
        if (event.key !== '.') autoAdvancedTo.current = null;

        switch (event.key) {
            case 'Tab': {
                // The four octets are one form field: Tab skips the rest of them.
                const sibling = wrapperRef.current
                    ? findFocusableOutside(wrapperRef.current, event.shiftKey ? -1 : 1)
                    : undefined;
                if (sibling) {
                    event.preventDefault();
                    sibling.focus();
                }
                return;
            }
            case 'ArrowLeft':
            case 'ArrowRight': {
                const atLeftEdge = event.key === 'ArrowLeft' && start === 0 && end === 0;
                const atRightEdge = event.key === 'ArrowRight' && start === input.value.length && end === input.value.length;
                if (event.key === 'ArrowLeft' && atLeftEdge && index > 0) {
                    event.preventDefault();
                    focusSegment(index - 1, 'end');
                } else if (event.key === 'ArrowRight' && atRightEdge && index < 3) {
                    event.preventDefault();
                    focusSegment(index + 1, 'start');
                }
                return;
            }
            case 'Home':
                event.preventDefault();
                focusSegment(0, 'start');
                return;
            case 'End':
                event.preventDefault();
                focusSegment(
                    segments.reduce((lastIndex, segment, position) => (segment ? position : lastIndex), 0),
                    'end',
                );
                return;
            case 'ArrowUp':
            case 'ArrowDown': {
                event.preventDefault();
                const delta = event.key === 'ArrowUp' ? 1 : -1;
                const current = Number(segments[index] || 0);
                replaceSegment(index, String(Math.min(255, Math.max(0, current + delta))));
                return;
            }
            case 'Backspace':
                // At the start of an octet Backspace eats the previous octet's
                // last digit, like a single text field would.
                if (start === 0 && end === 0 && index > 0) {
                    event.preventDefault();
                    const nextSegments = [...segments];
                    nextSegments[index - 1] = (segments[index - 1] ?? '').slice(0, -1);
                    onChange(serializeSegments(nextSegments));
                    focusSegment(index - 1, 'end');
                    // The value lands after the state update, so the caret is
                    // placed again once the octet is shorter.
                    requestAnimationFrame(() => focusSegment(index - 1, 'end'));
                }
                return;
            case '.': {
                const movedByFill = autoAdvancedTo.current === index;
                autoAdvancedTo.current = null;
                if (index < 3) {
                    // The dot closing a just-filled octet is the one that already
                    // moved on; any other dot moves to the next octet.
                    event.preventDefault();
                    if (!movedByFill) focusSegment(index + 1, 'start', true);
                }
                return;
            }
            default:
                return;
        }
    };

    return (
        <div className="ipv4-input-wrapper">
            <div
                ref={wrapperRef}
                className={inputClassName}
                onBlur={(event) => {
                    // Focus moving between the octets is still the same field,
                    // so it must not count as leaving it (the form validates
                    // when a field is left).
                    const next = event.relatedTarget;
                    if (next instanceof Node && event.currentTarget.contains(next)) {
                        event.stopPropagation();
                    }
                }}
                onCopy={handleCopy}
                // A press picks the octet the pointer points at, so the dot is
                // no longer the one that followed the automatic move.
                onPointerDown={() => {
                    autoAdvancedTo.current = null;
                }}
                role="group"
            >
                {segments.map((segment, index) => (
                    <React.Fragment key={index}>
                        {index > 0 && <span aria-hidden="true" className="ipv4-input-separator">.</span>}
                        <input
                            ref={(element) => { inputRefs.current[index] = element; }}
                            className="ipv4-input-segment"
                            disabled={disabled}
                            id={index === 0 ? id : id ? `${id}-octet-${index + 1}` : undefined}
                            inputMode="numeric"
                            maxLength={3}
                            name={index === 0 ? name : undefined}
                            onChange={(event) => handleSegmentChange(index, event.target.value)}
                            onKeyDown={(event) => handleKeyDown(event, index)}
                            onPaste={(event) => handlePaste(event, index)}
                            tabIndex={index === 0 ? undefined : -1}
                            type="text"
                            value={segment}
                        />
                    </React.Fragment>
                ))}
            </div>
        </div>
    );
};
