import React, { useState } from 'react';

export interface SettingRowProps {
  icon?: React.ReactNode;
  title: string;
  subtitle?: string;
  control?: React.ReactNode;
  children?: React.ReactNode;
  expandable?: boolean;
  defaultExpanded?: boolean;
}

export const SettingRow: React.FC<SettingRowProps> = ({
  icon,
  title,
  subtitle,
  control,
  children,
  expandable = false,
  defaultExpanded = false,
}) => {
  const [expanded, setExpanded] = useState(defaultExpanded);

  return (
    <div className={`win-setting-row ${expandable ? 'is-expandable' : ''}`}>
      <div className="win-setting-row-main" onClick={() => expandable && setExpanded(!expanded)}>
        {icon && <div className="win-setting-icon">{icon}</div>}
        <div className="win-setting-text">
          <div className="win-setting-title">{title}</div>
          {subtitle && <div className="win-setting-subtitle">{subtitle}</div>}
        </div>
        {control && <div className="win-setting-control" onClick={(e) => e.stopPropagation()}>{control}</div>}
        {expandable && (
          <button 
            className="win-setting-expand-btn" 
            aria-label="Expandir sección"
            onClick={(e) => {
              e.stopPropagation();
              setExpanded(!expanded);
            }}
          >
            <svg 
              className={`win-chevron ${expanded ? 'expanded' : ''}`} 
              viewBox="0 0 12 12" 
              width="12" 
              height="12" 
              fill="none" 
              stroke="currentColor"
            >
              <path d="M2.5 4.5L6 8L9.5 4.5" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </button>
        )}
      </div>
      {expandable && expanded && children && (
        <div className="win-setting-row-details">
          {children}
        </div>
      )}
    </div>
  );
};

interface SettingCardProps {
  title?: string;
  children: React.ReactNode;
}

export const SettingCard: React.FC<SettingCardProps> = ({ title, children }) => {
  return (
    <div className="win-card-group">
      {title && <h3 className="win-section-header">{title}</h3>}
      <div className="win-card">
        {children}
      </div>
    </div>
  );
};
