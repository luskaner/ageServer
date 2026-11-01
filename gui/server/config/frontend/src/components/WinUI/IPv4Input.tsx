import React, { useMemo, useRef } from 'react';

export interface IPv4InputProps {
    value: string;
    onChange: (value: string) => void;
    className?: string;
    disabled?: boolean;
    error?: string;
    id?: string;
    max?: string;
    min?: string;
    name?: string;
    placeholder?: string;
    showValidationMessage?: boolean;
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

function parseAddress(value: string): number[] | undefined {
    const segments = value.trim().split('.');
    if (segments.length !== 4 || segments.some((segment) => !/^(?:0|[1-9]\d{0,2})$/.test(segment))) {
        return undefined;
    }

    const octets = segments.map(Number);
    return octets.every((octet) => octet >= 0 && octet <= 255) ? octets : undefined;
}

function addressToNumber(address: number[]): number {
    return (((address[0] * 256 + address[1]) * 256 + address[2]) * 256) + address[3];
}

function getBounds(
    min: string | undefined,
    max: string | undefined,
): { min?: number; max?: number } {
    const parsedMin = parseAddress(min ?? '');
    const parsedMax = parseAddress(max ?? '');

    return {
        min: parsedMin ? addressToNumber(parsedMin) : undefined,
        max: parsedMax ? addressToNumber(parsedMax) : undefined,
    };
}

function getValidationMessage(
    value: string,
    min: string | undefined,
    max: string | undefined,
): string | undefined {
    const address = parseAddress(value);
    if (!address) return value ? 'Enter a valid IPv4 address.' : undefined;

    const number = addressToNumber(address);
    const bounds = getBounds(min, max);
    if (bounds.min !== undefined && number < bounds.min) return `Address must be at least ${min}.`;
    if (bounds.max !== undefined && number > bounds.max) return `Address must be at most ${max}.`;
    return undefined;
}

function parsePastedValue(value: string): string[] | undefined {
    if (!/^[\d.]+$/.test(value)) return undefined;

    if (value.includes('.')) {
        const segments = value.split('.');
        return segments.length <= 4 ? segments : undefined;
    }

    const segments: string[] = [];
    for (let index = 0; index < value.length; index += 3) {
        segments.push(value.slice(index, index + 3));
    }
    return segments.length <= 4 ? segments : undefined;
}

/**
 * Renders an IPv4 address as four numeric fields and normalizes pasted input.
 */
export const IPv4Input: React.FC<IPv4InputProps> = ({
    value,
    onChange,
    className = '',
    disabled = false,
    error,
    id,
    max,
    min,
    name,
    placeholder,
    showValidationMessage = true,
}) => {
    const inputRefs = useRef<Array<HTMLInputElement | null>>([]);
    const segments = useMemo(() => parseSegments(value), [value]);
    const validationMessage = error ?? getValidationMessage(value, min, max);
    const inputClassName = ['ipv4-input', validationMessage ? 'has-error' : '', className]
        .filter(Boolean)
        .join(' ');

    const focusSegment = (index: number) => {
        inputRefs.current[index]?.focus();
    };

    const handleSegmentChange = (index: number, nextValue: string) => {
        const digits = nextValue.replace(/\D/g, '').slice(0, 3);
        const nextSegments = [...segments];
        nextSegments[index] = digits;
        onChange(serializeSegments(nextSegments));

        if (digits.length >= 3 && index < 3) focusSegment(index + 1);
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
        focusSegment(Math.min(targetStart + pastedSegments.length - 1, 3));
    };

    const handleCopy = (event: React.ClipboardEvent<HTMLDivElement>) => {
        event.preventDefault();
        event.clipboardData.setData('text/plain', serializeSegments(segments));
    };

    const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>, index: number) => {
        if (event.key === '.' && index < 3) {
            event.preventDefault();
            focusSegment(index + 1);
        } else if (event.key === 'Backspace' && !segments[index] && index > 0) {
            focusSegment(index - 1);
        }
    };

    return (
        <div className="ipv4-input-wrapper">
            <div
                className={inputClassName}
                onCopy={handleCopy}
                role="group"
            >
                {segments.map((segment, index) => (
                    <React.Fragment key={index}>
                        {index > 0 && <span aria-hidden="true" className="ipv4-input-separator">.</span>}
                        <input
                            ref={(element) => { inputRefs.current[index] = element; }}
                            aria-invalid={Boolean(validationMessage)}
                            className="ipv4-input-segment"
                            disabled={disabled}
                            id={index === 0 ? id : id ? `${id}-octet-${index + 1}` : undefined}
                            inputMode="numeric"
                            maxLength={3}
                            name={index === 0 ? name : undefined}
                            onChange={(event) => handleSegmentChange(index, event.target.value)}
                            onKeyDown={(event) => handleKeyDown(event, index)}
                            onPaste={(event) => handlePaste(event, index)}
                            placeholder={index === 0 ? placeholder : undefined}
                            type="text"
                            value={segment}
                        />
                    </React.Fragment>
                ))}
            </div>
            {showValidationMessage && (
                <span className="ipv4-input-error">{validationMessage ?? '\u00a0'}</span>
            )}
        </div>
    );
};
