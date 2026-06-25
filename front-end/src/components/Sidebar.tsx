import React from 'react';

export type TabName =
  | 'dashboard'
  | 'check'
  | 'send'
  | 'quarantine'
  | 'gmail'
  | 'releases'
  | 'audit'
  | 'policies'
  | 'members'
  | 'settings'
  | 'plans';

export interface SidebarProps {
  activeTab: TabName;
  onTabChange: (tab: TabName) => void;
  role: string;
  collapsed: boolean;
  mobileOpen: boolean;
  onMobileClose: () => void;
  quarantineBadge: number;
  releasesBadge: number;
  email: string;
  onLogout: () => void;
}

interface NavItem {
  tab: TabName;
  label: string;
  ownerOnly?: boolean;
  icon: React.ReactNode;
}

const navItems: NavItem[] = [
  {
    tab: 'dashboard',
    label: 'Dashboard',
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <rect x="3" y="3" width="7" height="7" rx="1"/>
        <rect x="14" y="3" width="7" height="7" rx="1"/>
        <rect x="14" y="14" width="7" height="7" rx="1"/>
        <rect x="3" y="14" width="7" height="7" rx="1"/>
      </svg>
    ),
  },
  {
    tab: 'check',
    label: 'Check Email',
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <rect x="2" y="4" width="20" height="16" rx="2"/>
        <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>
      </svg>
    ),
  },
  {
    tab: 'send',
    label: 'Send Email',
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <path d="m22 2-7 20-4-9-9-4Z"/>
        <path d="M22 2 11 13"/>
      </svg>
    ),
  },
  {
    tab: 'quarantine',
    label: 'Quarantine',
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
      </svg>
    ),
  },
  {
    tab: 'gmail',
    label: 'Gmail',
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
        <polyline points="22,6 12,13 2,6"/>
      </svg>
    ),
  },
  {
    tab: 'releases',
    label: 'Review Requests',
    ownerOnly: true,
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/>
        <rect x="9" y="3" width="6" height="4" rx="1"/>
        <path d="m9 12 2 2 4-4"/>
      </svg>
    ),
  },
  {
    tab: 'audit',
    label: 'Audit Log',
    ownerOnly: true,
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <line x1="18" y1="20" x2="18" y2="10"/>
        <line x1="12" y1="20" x2="12" y2="4"/>
        <line x1="6" y1="20" x2="6" y2="16"/>
      </svg>
    ),
  },
  {
    tab: 'policies',
    label: 'Policies',
    ownerOnly: true,
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
        <polyline points="14 2 14 8 20 8"/>
        <line x1="16" y1="13" x2="8" y2="13"/>
        <line x1="16" y1="17" x2="8" y2="17"/>
      </svg>
    ),
  },
  {
    tab: 'members',
    label: 'Team',
    ownerOnly: true,
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
        <circle cx="9" cy="7" r="4"/>
        <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
        <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
      </svg>
    ),
  },
  {
    tab: 'settings',
    label: 'Settings',
    ownerOnly: false,
    icon: (
      <svg className="w-[18px] h-[18px] flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <circle cx="12" cy="12" r="3"/>
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
      </svg>
    ),
  },
];

export function Sidebar({
  activeTab,
  onTabChange,
  role,
  collapsed,
  mobileOpen,
  onMobileClose,
  quarantineBadge,
  releasesBadge,
  email,
  onLogout,
}: SidebarProps) {
  const isOwner = role === 'owner';
  const avatarLetter = email ? email[0].toUpperCase() : '?';

  const mainItems = navItems.filter((i) => !i.ownerOnly);
  const mgmtItems = navItems.filter((i) => i.ownerOnly);

  const sidebarWidth = collapsed ? '60px' : '232px';

  const overlayStyle: React.CSSProperties = {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.4)',
    zIndex: 199,
    display: mobileOpen ? 'block' : 'none',
  };

  const sidebarStyle: React.CSSProperties = {
    position: 'fixed',
    top: 0,
    left: 0,
    bottom: 0,
    width: sidebarWidth,
    background: '#fff',
    borderRight: '1px solid #f3f4f6',
    display: 'flex',
    flexDirection: 'column',
    zIndex: 200,
    transition: 'width 0.2s, transform 0.2s',
    overflow: 'hidden',
  };

  const renderItem = (item: NavItem) => {
    const isActive = activeTab === item.tab;
    const badge =
      item.tab === 'quarantine' && quarantineBadge > 0
        ? quarantineBadge
        : item.tab === 'releases' && releasesBadge > 0
        ? releasesBadge
        : 0;

    return (
      <a
        key={item.tab}
        href="#"
        onClick={(e) => {
          e.preventDefault();
          onTabChange(item.tab);
          onMobileClose();
        }}
        className={`flex items-center gap-2.5 mx-2 px-3 py-2 rounded-lg text-[13.5px] font-medium transition-colors no-underline ${
          isActive
            ? 'bg-green-50 text-green-800 font-semibold'
            : 'text-gray-500 hover:bg-gray-50 hover:text-gray-800'
        }`}
        style={{ textDecoration: 'none' }}
      >
        <span style={{ color: isActive ? '#3d9970' : undefined, flexShrink: 0 }}>
          {item.icon}
        </span>
        {!collapsed && <span className="whitespace-nowrap flex-1">{item.label}</span>}
        {!collapsed && badge > 0 && (
          <span className="ml-auto bg-red-500 text-white text-[10px] font-bold px-1.5 py-px rounded-full">
            {badge}
          </span>
        )}
      </a>
    );
  };

  return (
    <>
      <div style={overlayStyle} onClick={onMobileClose} />
      <aside style={sidebarStyle}>
        {/* Logo */}
        <div className="h-[58px] px-4 flex items-center gap-2.5 border-b border-gray-100 flex-shrink-0 overflow-hidden">
          <img src="/logo.png" className="w-7 h-7 flex-shrink-0" alt="Quarantio" />
          {!collapsed && (
            <span className="font-bold text-gray-900 text-[15px] whitespace-nowrap">
              Quarantio
            </span>
          )}
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto py-2">
          {!collapsed && (
            <div className="text-[10px] font-semibold uppercase tracking-widest text-gray-400 px-4 pt-3 pb-1">
              Main
            </div>
          )}
          {mainItems.map(renderItem)}

          {isOwner && (
            <>
              {!collapsed && (
                <div className="text-[10px] font-semibold uppercase tracking-widest text-gray-400 px-4 pt-4 pb-1">
                  Management
                </div>
              )}
              {mgmtItems.map(renderItem)}
            </>
          )}
        </nav>

        {/* Footer */}
        <div
          className="border-t border-gray-100 px-3.5 py-3 flex items-center gap-2.5 overflow-hidden flex-shrink-0"
          style={{ justifyContent: collapsed ? 'center' : undefined }}
        >
          <div className="w-8 h-8 rounded-full bg-brand text-white flex items-center justify-center text-xs font-bold flex-shrink-0">
            {avatarLetter}
          </div>
          {!collapsed && (
            <div className="flex-1 min-w-0">
              <div className="text-xs text-gray-500 truncate">{email}</div>
            </div>
          )}
          {!collapsed && (
            <button
              onClick={onLogout}
              className="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors flex-shrink-0"
              title="Sign out"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
                <polyline points="16 17 21 12 16 7"/>
                <line x1="21" y1="12" x2="9" y2="12"/>
              </svg>
            </button>
          )}
        </div>
      </aside>
    </>
  );
}
