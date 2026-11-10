import type { ReactNode } from 'react';
import { PPPoE as SessionsPPPoE } from '../../features/pppoe/pppoe';
import type { PPPoEProps } from '../../features/pppoe/pppoe';
import { PPPoESessionDetail as SessionDetail } from '../../features/pppoe/pppoe-session-screen';
import type { PPPoESessionDetailProps } from '../../features/pppoe/pppoe-session-props';
import { SessionLinkProvider } from '../../features/pppoe/session-link-slot';
import { RadioLinkPanel } from '../../features/radio/radio';

// A aba Enlace da sessão mostra o rádio; a página liga o painel ao encaixe do PPPoE.
function WithRadioLink(p: { children: ReactNode }) {
  return (
    <SessionLinkProvider
      render={(link) => <RadioLinkPanel session {...link} />}
    >
      {p.children}
    </SessionLinkProvider>
  );
}

export function PPPoE(p: PPPoEProps) {
  return (
    <WithRadioLink>
      <SessionsPPPoE {...p} />
    </WithRadioLink>
  );
}

export function PPPoESessionDetail(p: PPPoESessionDetailProps) {
  return (
    <WithRadioLink>
      <SessionDetail {...p} />
    </WithRadioLink>
  );
}
