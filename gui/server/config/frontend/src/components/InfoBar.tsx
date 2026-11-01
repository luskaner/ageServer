import React from 'react';
import { IconAlertTriangle, IconCircleX, IconInfoCircle } from '@tabler/icons-react';

export type InfoBarSeverity = 'info' | 'warning' | 'error';

export interface InfoBarProps {
    severity?: InfoBarSeverity;
    children: React.ReactNode;
}

const ICONS: Record<InfoBarSeverity, React.ReactNode> = {
    info: <IconInfoCircle size={16} stroke={1.8} />,
    warning: <IconAlertTriangle size={16} stroke={1.8} />,
    error: <IconCircleX size={16} stroke={1.8} />,
};

/**
 * Inline message in the shape Windows 11 gives its InfoBar: a tinted surface
 * with a hairline stroke, the severity icon and the text, kept to one line
 * block so it never competes with the settings cards around it. Every advisory
 * in the app goes through here, whether it comes from validation or from a
 * setting that will not apply by itself.
 */
export const InfoBar: React.FC<InfoBarProps> = ({ severity = 'info', children }) => (
    <div
        className={`app-infobar app-infobar--${severity}`}
        role={severity === 'error' ? 'alert' : 'status'}
    >
        <span className="app-infobar-icon" aria-hidden="true">
            {ICONS[severity]}
        </span>
        <div className="app-infobar-body">{children}</div>
    </div>
);
