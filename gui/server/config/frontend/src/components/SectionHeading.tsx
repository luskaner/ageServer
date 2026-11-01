import React from 'react';
import { Text } from '@mantine/core';

export type SectionHeadingLevel = 'section' | 'group';

export interface SectionHeadingProps {
    children: React.ReactNode;
    /**
     * `section` names a block of cards on a page ("Battle servers"); `group`
     * names a part of one card ("Startup", "Ports"). The two levels are the only
     * captions in the app, so a heading always reads the same way.
     */
    level?: SectionHeadingLevel;
    /** Adds the space a page-level section needs above it. */
    spaced?: boolean;
}

export const SectionHeading: React.FC<SectionHeadingProps> = ({
    children,
    level = 'group',
    spaced = false,
}) => (
    <Text
        size={level === 'section' ? 'sm' : 'xs'}
        fw={600}
        c={level === 'section' ? undefined : 'dimmed'}
        mt={spaced ? 'md' : 0}
        mb={level === 'section' ? 2 : 2}
    >
        {children}
    </Text>
);
