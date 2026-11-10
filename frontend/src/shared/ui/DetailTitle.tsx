import { createContext } from 'react';

export const DetailTitle = createContext<(title: string) => void>(() => {});
