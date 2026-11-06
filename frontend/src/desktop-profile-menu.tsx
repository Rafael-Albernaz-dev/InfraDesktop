import type { DesktopProps } from './desktop-model';
import { Button } from './ui/Button';
import { ShellIcon } from './ui/ShellIcon';
type ProfileProps = {
  p: DesktopProps;
  menu: string;
  setMenu: (value: string) => void;
  profileTrigger: React.RefObject<HTMLButtonElement | null>;
  profileMenu: React.RefObject<HTMLDivElement | null>;
};
function ProfileTrigger({ p, menu, setMenu, profileTrigger }: ProfileProps) {
  return (
    <Button
      ref={profileTrigger}
      onKeyDown={(event) => {
        if (event.key === 'ArrowDown') {
          event.preventDefault();
          setMenu('profile');
        }
      }}
      className="user"
      aria-label={`Conta: ${p.account}`}
      aria-haspopup="true"
      aria-expanded={menu === 'profile'}
      onClick={() => setMenu(menu === 'profile' ? '' : 'profile')}
    >
      <span className="av">{p.account.slice(0, 1).toUpperCase()}</span>
      <b>{p.account}</b>
      <ShellIcon name="chev" />
    </Button>
  );
}
function ProfileDropdown({ p, setMenu, profileMenu }: ProfileProps) {
  return (
    <div
      className="dd"
      role="menu"
      ref={profileMenu}
      onKeyDown={focusProfileItem}
    >
      <Button
        className="mi"
        disabled={p.busy}
        role="menuitem"
        onClick={() => {
          p.setTheme(p.theme === 'dark' ? 'light' : 'dark');
          setMenu('');
        }}
      >
        <ShellIcon name="moon" />
        Tema {p.theme === 'dark' ? 'claro' : 'escuro'}
      </Button>
      <hr />
      <Button
        className="mi sair"
        role="menuitem"
        onClick={() => {
          setMenu('');
          p.logout();
        }}
      >
        <ShellIcon name="out" />
        Sair da conta
      </Button>
    </div>
  );
}
export function DesktopProfileMenu(p: ProfileProps) {
  return (
    <div className="wrap">
      <ProfileTrigger {...p} />
      {p.menu === 'profile' && <ProfileDropdown {...p} />}
    </div>
  );
}
function focusProfileItem(event: React.KeyboardEvent<HTMLDivElement>) {
  const items = Array.from(
    event.currentTarget.querySelectorAll<HTMLElement>('[role=menuitem]'),
  );
  const index = items.indexOf(event.target as HTMLElement);
  const next = menuFocusIndex(event.key, index, items.length);
  if (next >= 0) {
    event.preventDefault();
    items[next]?.focus();
  }
}
function menuFocusIndex(key: string, index: number, length: number) {
  if (key === 'ArrowDown') return (index + 1) % length;
  if (key === 'ArrowUp') return (index + length - 1) % length;
  if (key === 'Home') return 0;
  if (key === 'End') return length - 1;
  return -1;
}
