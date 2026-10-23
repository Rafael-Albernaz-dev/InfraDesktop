import { useRef, useState } from 'react';
import { access } from './presentation';
import { recoveryActionMessage } from './microcopy';
export function RecoveryActions({ busy }: { busy: boolean }) {
  const [feedback, setFeedback] = useState('');
  const [pending, setPending] = useState(false);
  const running = useRef(false);
  async function act(action: 'copy' | 'save') {
    if (running.current) return;
    running.current = true;
    setPending(true);
    setFeedback('');
    try {
      setFeedback(
        recoveryActionMessage(
          await (action === 'copy'
            ? access.copyRecovery()
            : access.saveRecovery()),
          action,
        ),
      );
    } catch {
      setFeedback(recoveryActionMessage('UNAVAILABLE', action));
    } finally {
      running.current = false;
      setPending(false);
    }
  }
  return (
    <div className="recovery-actions">
      <p>
        O arquivo contém um segredo em texto claro. Guarde-o em local seguro.
      </p>
      <div className="dialog-actions">
        <button
          type="button"
          className="btn"
          disabled={busy || pending}
          onClick={() => void act('copy')}
        >
          Copiar código
        </button>
        <button
          type="button"
          className="btn"
          disabled={busy || pending}
          onClick={() => void act('save')}
        >
          Salvar .txt
        </button>
      </div>
      {feedback && <p role="status">{feedback}</p>}
    </div>
  );
}
