import React from 'react';

interface ToggleSwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: string;
  disabled?: boolean;
}

export const ToggleSwitch: React.FC<ToggleSwitchProps> = ({ checked, onChange, label, disabled }) => {
  return (
    <label className={`win-toggle-container ${disabled ? 'disabled' : ''}`}>
      <span className="win-toggle-status">{label || (checked ? 'Activado' : 'Desactivado')}</span>
      <div 
        className={`win-toggle ${checked ? 'checked' : ''}`}
        onClick={() => !disabled && onChange(!checked)}
        tabIndex={disabled ? -1 : 0}
        role="switch"
        aria-checked={checked}
        onKeyDown={(e) => {
          if (!disabled && (e.key === ' ' || e.key === 'Enter')) {
            e.preventDefault();
            onChange(!checked);
          }
        }}
      >
        <div className="win-toggle-thumb" />
      </div>
    </label>
  );
};
