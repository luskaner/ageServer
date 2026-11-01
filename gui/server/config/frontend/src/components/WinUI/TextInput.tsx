import React from 'react';

interface TextInputProps {
  value: string | number;
  onChange: (val: string) => void;
  placeholder?: string;
  type?: 'text' | 'number';
  error?: string;
  disabled?: boolean;
}

export const TextInput: React.FC<TextInputProps> = ({
  value,
  onChange,
  placeholder,
  type = 'text',
  error,
  disabled,
}) => {
  return (
    <div className="win-input-wrapper">
      <input
        type={type}
        className={`win-input ${error ? 'has-error' : ''}`}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
      />
      {error && <span className="win-input-error-msg">{error}</span>}
    </div>
  );
};
