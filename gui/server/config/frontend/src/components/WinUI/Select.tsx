import React from 'react';

interface Option {
  value: string;
  label: string;
}

interface SelectProps {
  value: string;
  options: Option[];
  onChange: (val: string) => void;
  disabled?: boolean;
}

export const Select: React.FC<SelectProps> = ({ value, options, onChange, disabled }) => {
  return (
    <div className="win-select-wrapper">
      <select
        className="win-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
      >
        {options.map((opt) => (
          <option key={opt.value} value={opt.value} className="win-select-option">
            {opt.label}
          </option>
        ))}
      </select>
      <svg className="win-select-arrow" viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor">
        <path d="M2.5 4.5L6 8L9.5 4.5" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    </div>
  );
};
