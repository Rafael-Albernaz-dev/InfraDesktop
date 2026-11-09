import type { FormEvent } from 'react';
import { useState } from 'react';

export type AccessProps = {
  theme?: string;
  setTheme?: (theme: string) => void;
  setup: boolean;
  busy: boolean;
  connected: boolean;
  error: string;
  recovery: string;
  resetVerification?: boolean;
  acknowledge: () => void;
  contextChanged?: () => void;
  initialName?: string;
  initialMode?: string;
  submit: (
    mode: string,
    name: string,
    password: string,
    code: string,
    remember: boolean,
  ) => void;
};

export function useAccessFields(p: AccessProps) {
  const [mode, setMode] = useState(p.initialMode || 'login');
  const [name, setName] = useState(p.initialName || '');
  const [password, setPassword] = useState(''),
    [confirm, setConfirm] = useState('');
  const [code, setCode] = useState(''),
    [remember, setRemember] = useState(true);
  const [show, setShow] = useState(false),
    [localError, setLocalError] = useState('');
  function change(next: string) {
    if ((p.busy && mode !== 'login') || p.recovery) return;
    p.contextChanged?.();
    setMode(next);
    setPassword('');
    setConfirm('');
    setCode('');
    setShow(false);
    setLocalError('');
  }
  function send(event: FormEvent) {
    event.preventDefault();
    if (p.busy || p.recovery) return;
    if ((mode === 'register' || mode === 'reset') && password !== confirm) {
      setLocalError('As senhas não conferem.');
      return;
    }
    p.submit(mode, name, password, code, remember);
    setPassword('');
    setConfirm('');
    setCode('');
    setLocalError('');
  }
  return {
    mode,
    name,
    setName,
    password,
    setPassword,
    confirm,
    setConfirm,
    code,
    setCode,
    remember,
    setRemember,
    show,
    setShow,
    localError,
    finish: () => {
      setMode('login');
      setPassword('');
      setConfirm('');
      setCode('');
      setShow(false);
    },
    change,
    send,
  };
}
