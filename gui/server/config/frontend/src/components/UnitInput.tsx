import React, { useState } from 'react';
import { Group, Select, TextInput } from '@mantine/core';

/** A unit the split input can show. */
export interface UnitInputUnit {
    /** Text shown in the dropdown, such as "MB" or "days". */
    label: string;
    /** Unit written to the configuration; a label may stand for another unit. */
    storedUnit: string;
    /** How many stored units the label is worth (a day is 24 hours, say). */
    factor?: number;
}

export interface UnitInputProps {
    /** Value as the configuration stores it, number and unit together ("32MB", "12h"). */
    value: string;
    /** Receives the value with the unit written the way the configuration stores it. */
    onChange: (value: string) => void;
    /** Units the dropdown offers; when two share a stored unit, list the plain one first. */
    units: readonly UnitInputUnit[];
    /** Accessible name for the pair of controls. */
    ariaLabel: string;
    /** Called when the unit changes, which finishes the value by itself. */
    onCommit?: () => void;
    disabled?: boolean;
}

/** Splits a stored value into the number the user edits and the unit it is written with. */
function splitValue(
    value: string,
    units: readonly UnitInputUnit[],
): { number: string; unit: UnitInputUnit | null } {
    const match = /^(\d*\.?\d*)\s*(.*)$/.exec(value.trim());
    const number = match?.[1] ?? '';
    const suffix = match?.[2]?.trim() ?? '';
    if (!suffix) return { number, unit: null };
    const known = units.find((candidate) => candidate.storedUnit === suffix)
        ?? units.find((candidate) => candidate.label === suffix);
    // A file may hold a unit the dropdown does not list (KB, m): it becomes an
    // option of its own, so opening the file never rewrites the value.
    return { number, unit: known ?? { label: suffix, storedUnit: suffix, factor: 1 } };
}

/** Joins a number and a unit into the value the configuration stores. */
function joinValue(number: string, unit: UnitInputUnit | null): string {
    if (!unit) return number;
    const factor = unit.factor ?? 1;
    if (factor === 1 || number === '') return `${number}${unit.storedUnit}`;
    const scaled = Math.round(Number.parseFloat(number) * factor * 1000) / 1000;
    return Number.isNaN(scaled) ? unit.storedUnit : `${scaled}${unit.storedUnit}`;
}

/**
 * Number with its unit in a split dropdown beside it ("32 [MB]"), the way
 * Windows edits sizes and intervals: the unit is picked from a list, never
 * typed.
 */
export const UnitInput: React.FC<UnitInputProps> = ({
    value,
    onChange,
    units,
    ariaLabel,
    onCommit,
    disabled = false,
}) => {
    // The number and the unit are kept here rather than read back from the
    // stored value: a unit such as days is written as hours, so the stored
    // value cannot tell which of the two the user is looking at.
    const [number, setNumber] = useState(() => splitValue(value, units).number);
    const [unit, setUnit] = useState<UnitInputUnit | null>(() => splitValue(value, units).unit);
    // Mantine refuses two options with the same value: a unit of its own is
    // added only while the list does not show it yet.
    const options = unit && !units.some((candidate) => candidate.label === unit.label)
        ? [...units, unit]
        : units;
    // Wide enough for the longest label the dropdown shows ("hours", "days").
    const unitWidth = Math.max(84, ...options.map((option) => option.label.length * 8 + 52));

    const changeNumber = (nextNumber: string) => {
        const kept = nextNumber.replace(/[^\d]/g, '');
        setNumber(kept);
        onChange(joinValue(kept, unit));
    };

    const changeUnit = (label: string | null) => {
        const nextUnit = options.find((candidate) => candidate.label === label) ?? null;
        setUnit(nextUnit);
        onChange(joinValue(number, nextUnit));
        // Picking a unit finishes the value, so it is validated right away.
        onCommit?.();
    };

    return (
        <Group gap={0} wrap="nowrap" align="center">
            <TextInput
                type="number"
                min={0}
                step={1}
                value={number}
                onChange={(event) => changeNumber(event.currentTarget.value)}
                // A number field reports an empty value while what is typed
                // cannot be a number yet ("12.", "-", "1e"), so those keys are
                // kept out and the digits already there stay.
                onKeyDown={(event) => {
                    if (event.key.length === 1 && !/\d/.test(event.key) && !event.ctrlKey && !event.metaKey) {
                        event.preventDefault();
                    }
                }}
                aria-label={`${ariaLabel} value`}
                disabled={disabled}
                w={96}
                styles={{ input: { borderTopRightRadius: 0, borderBottomRightRadius: 0 } }}
            />
            <Select
                data={options.map((option) => ({ value: option.label, label: option.label }))}
                value={unit?.label ?? null}
                onChange={changeUnit}
                aria-label={`${ariaLabel} unit`}
                placeholder="Unit"
                allowDeselect={false}
                comboboxProps={{ withinPortal: true }}
                disabled={disabled}
                w={unitWidth}
                styles={{
                    input: {
                        borderTopLeftRadius: 0,
                        borderBottomLeftRadius: 0,
                        borderLeftWidth: 0,
                    },
                }}
            />
        </Group>
    );
};
