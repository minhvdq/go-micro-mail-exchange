import React, { useState } from 'react';
import { TENANT_URL } from '../config';

const G = `${TENANT_URL}/auth/google/login`;

export function Landing() {
  const [openFaq, setOpenFaq] = useState<number | null>(null);

  return (
    <div style={{ fontFamily: '-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif', color: '#0f1f17', margin: 0, overflowX: 'hidden' }}>
      <style>{`
        :root {
          --g50:#f0faf5; --g100:#dcf0e7; --g200:#b3dfc8; --g400:#52ad82;
          --g500:#3d9970; --g600:#2f7a58; --g700:#1e5c3f; --g900:#0b2419;
        }
        *{box-sizing:border-box;margin:0;padding:0;}
        a{text-decoration:none;}

        /* ── Nav ── */
        .lp-nav{position:sticky;top:0;z-index:200;background:rgba(255,255,255,.92);backdrop-filter:blur(12px);border-bottom:1px solid var(--g100);height:62px;display:flex;align-items:center;padding:0 5%;}
        .lp-nav-inner{max-width:1140px;width:100%;margin:0 auto;display:flex;align-items:center;justify-content:space-between;}
        .lp-logo{display:flex;align-items:center;gap:9px;font-weight:800;font-size:1.1rem;color:var(--g700);letter-spacing:-.02em;}
        .lp-logo img{width:30px;height:30px;object-fit:contain;}
        .lp-nav-links{display:flex;align-items:center;gap:28px;}
        .lp-nav-links a{font-size:.875rem;color:#4a6858;font-weight:500;transition:color .15s;}
        .lp-nav-links a:hover{color:var(--g700);}
        .lp-nav-ctas{display:flex;align-items:center;gap:10px;}
        .btn-ghost{padding:7px 18px;border-radius:8px;font-size:.85rem;font-weight:500;color:var(--g600);border:1.5px solid var(--g200);background:transparent;cursor:pointer;transition:all .15s;}
        .btn-ghost:hover{background:var(--g50);border-color:var(--g400);}
        .btn-primary{padding:8px 20px;border-radius:8px;font-size:.85rem;font-weight:600;background:var(--g500);color:#fff;border:none;cursor:pointer;transition:all .15s;display:inline-flex;align-items:center;gap:7px;}
        .btn-primary:hover{background:var(--g600);transform:translateY(-1px);box-shadow:0 4px 14px rgba(61,153,112,.35);}

        /* ── Hero ── */
        .lp-hero{background:linear-gradient(160deg,var(--g900) 0%,var(--g700) 45%,var(--g500) 100%);padding:90px 5% 0;text-align:center;position:relative;overflow:hidden;}
        .lp-hero::before{content:'';position:absolute;inset:0;background:radial-gradient(ellipse 80% 60% at 50% 120%,rgba(92,184,138,.18) 0%,transparent 70%);}
        .hero-eyebrow{display:inline-flex;align-items:center;gap:6px;background:rgba(255,255,255,.12);border:1px solid rgba(255,255,255,.2);border-radius:999px;padding:5px 14px;font-size:.78rem;font-weight:600;color:rgba(255,255,255,.9);letter-spacing:.03em;text-transform:uppercase;margin-bottom:28px;}
        .hero-eyebrow span{width:6px;height:6px;border-radius:50%;background:#5cb88a;display:inline-block;}
        .lp-hero h1{font-size:clamp(2.2rem,5.5vw,3.8rem);font-weight:900;color:#fff;line-height:1.1;letter-spacing:-.04em;margin-bottom:22px;position:relative;}
        .lp-hero h1 em{font-style:normal;background:linear-gradient(90deg,#a8edca,#5cb88a);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;}
        .lp-hero-sub{font-size:1.1rem;color:rgba(255,255,255,.75);max-width:560px;margin:0 auto 40px;line-height:1.65;position:relative;}
        .hero-ctas{display:flex;gap:12px;justify-content:center;flex-wrap:wrap;position:relative;margin-bottom:56px;}
        .btn-hero-main{display:inline-flex;align-items:center;gap:10px;background:#fff;color:var(--g800);padding:14px 28px;border-radius:12px;font-size:.975rem;font-weight:700;transition:all .18s;border:none;cursor:pointer;}
        .btn-hero-main:hover{background:var(--g50);transform:translateY(-2px);box-shadow:0 8px 24px rgba(0,0,0,.18);}
        .btn-hero-outline{display:inline-flex;align-items:center;gap:8px;background:rgba(255,255,255,.1);color:#fff;padding:14px 24px;border-radius:12px;font-size:.975rem;font-weight:500;border:1.5px solid rgba(255,255,255,.25);cursor:pointer;transition:all .18s;}
        .btn-hero-outline:hover{background:rgba(255,255,255,.18);}

        /* ── Product mockup ── */
        .mockup-wrap{position:relative;max-width:900px;margin:0 auto;padding:0 5%;}
        .mockup{background:#fff;border-radius:14px 14px 0 0;border:1px solid rgba(255,255,255,.15);border-bottom:none;box-shadow:0 -8px 60px rgba(0,0,0,.4),0 -2px 12px rgba(0,0,0,.2);overflow:hidden;}
        .mockup-bar{background:#f6f8fa;border-bottom:1px solid #e5e7eb;padding:10px 16px;display:flex;align-items:center;gap:8px;}
        .mockup-dot{width:10px;height:10px;border-radius:50%;}
        .mockup-title{font-size:.72rem;font-weight:600;color:#6b7280;margin-left:6px;letter-spacing:.02em;}
        .mockup-body{display:flex;min-height:240px;}
        .mockup-sidebar{width:180px;border-right:1px solid #f3f4f6;background:#fff;padding:12px 0;flex-shrink:0;}
        .ms-item{padding:8px 14px;font-size:.75rem;color:#9ca3af;display:flex;align-items:center;gap:8px;}
        .ms-item.active{color:#1e5c3f;background:#f0faf5;font-weight:600;}
        .ms-dot{width:7px;height:7px;border-radius:50%;background:currentColor;flex-shrink:0;}
        .mockup-content{flex:1;padding:16px 20px;}
        .mc-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:12px;}
        .mc-title{font-size:.8rem;font-weight:700;color:#111827;}
        .mc-badge{font-size:.68rem;font-weight:600;padding:2px 8px;border-radius:999px;}
        .mc-row{display:flex;align-items:center;gap:10px;padding:9px 10px;border-radius:8px;margin-bottom:6px;border:1px solid #f3f4f6;background:#fff;}
        .mc-row.high{border-color:#fee2e2;background:#fff8f8;}
        .mc-row.medium{border-color:#fef3c7;background:#fffdf0;}
        .mc-row.low{border-color:#dcfce7;background:#f8fff9;}
        .mc-verdict{font-size:.65rem;font-weight:700;padding:2px 7px;border-radius:999px;flex-shrink:0;}
        .v-high{background:#fee2e2;color:#b91c1c;}
        .v-medium{background:#fef3c7;color:#92400e;}
        .v-low{background:#dcfce7;color:#166534;}
        .mc-subject{font-size:.75rem;font-weight:500;color:#374151;flex:1;}
        .mc-from{font-size:.68rem;color:#9ca3af;}

        /* ── Sections ── */
        .lp-section{padding:88px 5%;}
        .lp-inner{max-width:1140px;margin:0 auto;}
        .section-tag{font-size:.75rem;font-weight:700;text-transform:uppercase;letter-spacing:.1em;color:var(--g500);margin-bottom:12px;}
        .section-h2{font-size:clamp(1.7rem,3.5vw,2.4rem);font-weight:800;color:var(--g900);letter-spacing:-.03em;line-height:1.2;margin-bottom:16px;}
        .section-sub{font-size:1rem;color:#5a7a6a;line-height:1.7;max-width:540px;}

        /* ── Stats ── */
        .stats-bar{background:var(--g50);border-top:1px solid var(--g100);border-bottom:1px solid var(--g100);padding:36px 5%;}
        .stats-inner{max-width:1140px;margin:0 auto;display:grid;grid-template-columns:repeat(4,1fr);gap:24px;text-align:center;}
        .stat-num{font-size:2rem;font-weight:900;color:var(--g700);letter-spacing:-.03em;line-height:1;}
        .stat-label{font-size:.8rem;color:#6b8c7a;margin-top:4px;}

        /* ── Features ── */
        .feat-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;margin-top:52px;}
        .feat-card{background:#fff;border:1px solid var(--g100);border-radius:16px;padding:28px 24px;transition:all .2s;}
        .feat-card:hover{box-shadow:0 8px 32px rgba(61,153,112,.1);transform:translateY(-2px);border-color:var(--g200);}
        .feat-icon{width:44px;height:44px;border-radius:12px;background:var(--g50);border:1px solid var(--g100);display:flex;align-items:center;justify-content:center;font-size:1.25rem;margin-bottom:18px;}
        .feat-card h4{font-size:.975rem;font-weight:700;color:var(--g900);margin-bottom:8px;}
        .feat-card p{font-size:.875rem;color:#5a7a6a;line-height:1.65;}

        /* ── How it works ── */
        .hiw-grid{display:grid;grid-template-columns:1fr 1fr;gap:64px;align-items:center;margin-top:52px;}
        .hiw-steps{display:flex;flex-direction:column;gap:28px;}
        .hiw-step{display:flex;gap:16px;align-items:flex-start;}
        .hiw-num{width:36px;height:36px;border-radius:50%;background:var(--g500);color:#fff;font-weight:800;font-size:.85rem;display:flex;align-items:center;justify-content:center;flex-shrink:0;margin-top:2px;}
        .hiw-step h5{font-size:1rem;font-weight:700;color:var(--g900);margin-bottom:4px;}
        .hiw-step p{font-size:.875rem;color:#5a7a6a;line-height:1.6;}
        .hiw-visual{background:var(--g50);border:1px solid var(--g100);border-radius:16px;padding:28px;display:flex;flex-direction:column;gap:12px;}
        .hiw-email{background:#fff;border:1px solid var(--g100);border-radius:10px;padding:14px 16px;}
        .hiw-email-top{display:flex;align-items:center;justify-content:space-between;margin-bottom:6px;}
        .hiw-email-subject{font-size:.8rem;font-weight:600;color:#374151;}
        .hiw-email-from{font-size:.7rem;color:#9ca3af;}
        .hiw-email-body{font-size:.75rem;color:#6b7280;line-height:1.5;}
        .hiw-arrow{text-align:center;font-size:1.2rem;color:var(--g400);}
        .hiw-verdict{background:var(--g50);border:1.5px solid var(--g200);border-radius:10px;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;}
        .hiw-verdict-label{font-size:.75rem;font-weight:700;color:var(--g700);}
        .hiw-verdict-badge{font-size:.7rem;font-weight:700;padding:3px 10px;border-radius:999px;background:#fee2e2;color:#b91c1c;}

        /* ── Pricing ── */
        .pricing-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;margin-top:52px;}
        .plan-card{background:#fff;border:1.5px solid var(--g100);border-radius:18px;padding:32px 28px;position:relative;transition:all .2s;}
        .plan-card:hover{box-shadow:0 8px 32px rgba(61,153,112,.12);}
        .plan-card.popular{border-color:var(--g500);box-shadow:0 0 0 3px rgba(61,153,112,.1);}
        .popular-badge{position:absolute;top:-14px;left:50%;transform:translateX(-50%);background:var(--g500);color:#fff;font-size:.72rem;font-weight:700;padding:4px 16px;border-radius:999px;white-space:nowrap;}
        .plan-name{font-size:.85rem;font-weight:700;color:#6b8c7a;text-transform:uppercase;letter-spacing:.06em;margin-bottom:8px;}
        .plan-price{font-size:2.6rem;font-weight:900;color:var(--g900);letter-spacing:-.04em;line-height:1;}
        .plan-price sup{font-size:1.1rem;font-weight:600;vertical-align:top;margin-top:8px;}
        .plan-price sub{font-size:.9rem;font-weight:400;color:#6b8c7a;}
        .plan-tagline{font-size:.825rem;color:#6b8c7a;margin:8px 0 24px;}
        .plan-divider{border:none;border-top:1px solid var(--g100);margin:20px 0;}
        .plan-features{list-style:none;display:flex;flex-direction:column;gap:10px;margin-bottom:28px;}
        .plan-features li{font-size:.85rem;color:#3d5249;display:flex;align-items:flex-start;gap:8px;}
        .plan-features li::before{content:"✓";color:var(--g500);font-weight:700;flex-shrink:0;margin-top:1px;}
        .plan-cta{display:block;text-align:center;padding:12px;border-radius:10px;font-size:.9rem;font-weight:600;transition:all .18s;cursor:pointer;}
        .plan-cta-outline{border:1.5px solid var(--g200);color:var(--g600);background:transparent;}
        .plan-cta-outline:hover{border-color:var(--g500);color:var(--g700);background:var(--g50);}
        .plan-cta-solid{background:var(--g500);color:#fff;border:none;}
        .plan-cta-solid:hover{background:var(--g600);box-shadow:0 4px 14px rgba(61,153,112,.35);}
        .plan-note{font-size:.72rem;color:#9ca3af;text-align:center;margin-top:10px;}

        /* ── FAQ ── */
        .faq-list{max-width:700px;margin:48px auto 0;display:flex;flex-direction:column;gap:2px;}
        .faq-item{border-radius:12px;overflow:hidden;border:1px solid var(--g100);}
        .faq-q{display:flex;align-items:center;justify-content:space-between;padding:18px 22px;background:#fff;cursor:pointer;font-size:.95rem;font-weight:600;color:var(--g900);gap:16px;transition:background .15s;user-select:none;}
        .faq-q:hover{background:var(--g50);}
        .faq-q.open{background:var(--g50);color:var(--g700);}
        .faq-chevron{flex-shrink:0;transition:transform .2s;color:var(--g400);}
        .faq-chevron.open{transform:rotate(180deg);}
        .faq-a{padding:0 22px 18px;background:var(--g50);font-size:.875rem;color:#5a7a6a;line-height:1.7;}

        /* ── CTA ── */
        .lp-cta{background:linear-gradient(135deg,var(--g900) 0%,var(--g700) 60%,var(--g500) 100%);padding:88px 5%;text-align:center;}
        .lp-cta h2{font-size:clamp(1.8rem,4vw,2.8rem);font-weight:900;color:#fff;letter-spacing:-.03em;margin-bottom:16px;}
        .lp-cta p{font-size:1rem;color:rgba(255,255,255,.75);margin-bottom:36px;}

        /* ── Footer ── */
        lp-footer{background:#fff;border-top:1px solid var(--g100);}
        .footer-inner{max-width:1140px;margin:0 auto;padding:36px 5%;display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:16px;}
        .footer-logo{display:flex;align-items:center;gap:8px;font-weight:700;font-size:.9rem;color:var(--g700);}
        .footer-logo img{width:22px;height:22px;object-fit:contain;}
        .footer-links{display:flex;gap:24px;}
        .footer-links a{font-size:.8rem;color:#6b8c7a;transition:color .15s;}
        .footer-links a:hover{color:var(--g600);}
        .footer-copy{font-size:.78rem;color:#9cb8a8;}

        @media(max-width:900px){
          .feat-grid{grid-template-columns:1fr 1fr;}
          .hiw-grid{grid-template-columns:1fr;}
          .hiw-visual{display:none;}
          .pricing-grid{grid-template-columns:1fr;}
          .stats-inner{grid-template-columns:repeat(2,1fr);}
          .lp-nav-links{display:none;}
        }
        @media(max-width:600px){
          .feat-grid{grid-template-columns:1fr;}
          .lp-hero{padding:64px 5% 0;}
          .lp-section{padding:56px 5%;}
          .mockup-sidebar{display:none;}
        }
      `}</style>

      {/* ── Nav ── */}
      <nav className="lp-nav">
        <div className="lp-nav-inner">
          <a href="/" className="lp-logo">
            <img src="/logo.png" alt="Quarantio" />
            Quarantio
          </a>
          <div className="lp-nav-links">
            <a href="#features">Features</a>
            <a href="#how">How it works</a>
            <a href="#pricing">Pricing</a>
            <a href="#faq">FAQ</a>
          </div>
          <div className="lp-nav-ctas">
            <a href={G} className="btn-ghost">Sign In</a>
            <a href={G} className="btn-primary">
              <GoogleIcon size={15} />
              Get Started Free
            </a>
          </div>
        </div>
      </nav>

      {/* ── Hero ── */}
      <section className="lp-hero">
        <div className="hero-eyebrow"><span /> AI-Powered Email Compliance</div>
        <h1>Stop threats before<br />they hit the <em>inbox</em></h1>
        <p className="lp-hero-sub">
          Quarantio scans every inbound email against your compliance policies in real-time.
          Policy violations are quarantined automatically — no MX changes, no IT tickets.
        </p>
        <div className="hero-ctas">
          <a href={G} className="btn-hero-main">
            <GoogleIcon size={18} />
            Start free 14-day trial
          </a>
          <a href="#how" className="btn-hero-outline">See how it works</a>
        </div>

        {/* Product mockup */}
        <div className="mockup-wrap">
          <div className="mockup">
            <div className="mockup-bar">
              <div className="mockup-dot" style={{ background: '#ff5f57' }} />
              <div className="mockup-dot" style={{ background: '#ffbd2e' }} />
              <div className="mockup-dot" style={{ background: '#28c840' }} />
              <span className="mockup-title">Quarantio — Quarantine</span>
            </div>
            <div className="mockup-body">
              <div className="mockup-sidebar">
                {[
                  { label: 'Dashboard', active: false },
                  { label: 'Check Email', active: false },
                  { label: 'Quarantine', active: true },
                  { label: 'Audit Log', active: false },
                  { label: 'Policies', active: false },
                  { label: 'Team', active: false },
                  { label: 'Settings', active: false },
                ].map((i) => (
                  <div key={i.label} className={`ms-item${i.active ? ' active' : ''}`}>
                    <div className="ms-dot" />
                    {i.label}
                  </div>
                ))}
              </div>
              <div className="mockup-content">
                <div className="mc-header">
                  <span className="mc-title">Quarantined Emails</span>
                  <span className="mc-badge" style={{ background: '#fee2e2', color: '#b91c1c' }}>3 pending review</span>
                </div>
                {[
                  { verdict: 'HIGH', vClass: 'v-high', rowClass: 'high', subject: 'Urgent: Wire transfer required immediately', from: 'ceo-impersonator@gmail.com' },
                  { verdict: 'MEDIUM', vClass: 'v-medium', rowClass: 'medium', subject: 'Re: Q4 numbers — see attached report', from: 'unknown@external-domain.io' },
                  { verdict: 'LOW', vClass: 'v-low', rowClass: 'low', subject: 'Partnership opportunity — limited time offer', from: 'marketing@bulksender.net' },
                ].map((r) => (
                  <div key={r.subject} className={`mc-row ${r.rowClass}`}>
                    <span className={`mc-verdict ${r.vClass}`}>{r.verdict}</span>
                    <span className="mc-subject">{r.subject}</span>
                    <span className="mc-from">{r.from}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Stats ── */}
      <div className="stats-bar">
        <div className="stats-inner">
          {[
            { num: '<2s', label: 'Average scan time per email' },
            { num: '99%+', label: 'Policy violation detection rate' },
            { num: '0', label: 'MX record changes required' },
            { num: '14', label: 'Days free on Starter trial' },
          ].map((s) => (
            <div key={s.label}>
              <div className="stat-num">{s.num}</div>
              <div className="stat-label">{s.label}</div>
            </div>
          ))}
        </div>
      </div>

      {/* ── Features ── */}
      <section className="lp-section" id="features" style={{ background: '#fff' }}>
        <div className="lp-inner">
          <div style={{ textAlign: 'center' }}>
            <p className="section-tag">Features</p>
            <h2 className="section-h2">Everything you need to stay compliant</h2>
            <p className="section-sub" style={{ margin: '0 auto' }}>
              From real-time interception to detailed audit logs — built for compliance teams
              who can't afford to miss a policy violation.
            </p>
          </div>
          <div className="feat-grid">
            {[
              {
                icon: '🛡️',
                title: 'Auto Guard',
                desc: 'Emails are scanned the moment they arrive via Gmail. Violations are quarantined before the user ever sees them — zero inbox disruption.',
              },
              {
                icon: '🤖',
                title: 'AI Compliance Engine',
                desc: 'Upload your policy documents and our Mistral-powered engine learns your rules. Every email is checked against your specific requirements.',
              },
              {
                icon: '🔍',
                title: 'Quarantine & Review',
                desc: 'Flagged emails land in a secure, team-visible quarantine. Admins review, release, or permanently reject with one click and a full audit trail.',
              },
              {
                icon: '📊',
                title: 'Audit Logs',
                desc: 'Every scanned email generates a timestamped log with verdict, detected violations, and AI reasoning. Export anytime for compliance reporting.',
              },
              {
                icon: '👥',
                title: 'Team Management',
                desc: 'Invite your team, assign roles, and get real-time compliance visibility across all connected mailboxes. Owner controls who sees what.',
              },
              {
                icon: '⚡',
                title: 'Instant Setup',
                desc: 'Sign in with Google, connect your mailbox, start your trial. No MX changes, no forwarding rules, no IT department required.',
              },
            ].map((f) => (
              <div key={f.title} className="feat-card">
                <div className="feat-icon">{f.icon}</div>
                <h4>{f.title}</h4>
                <p>{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── How it works ── */}
      <section className="lp-section" id="how" style={{ background: 'var(--g50)', borderTop: '1px solid var(--g100)', borderBottom: '1px solid var(--g100)' }}>
        <div className="lp-inner">
          <p className="section-tag">How it works</p>
          <h2 className="section-h2">Up and running in minutes</h2>
          <div className="hiw-grid">
            <div className="hiw-steps">
              {[
                {
                  n: 1,
                  title: 'Sign in with Google',
                  desc: 'Your account and organization are created automatically. No forms, no passwords — one click and you\'re in.',
                },
                {
                  n: 2,
                  title: 'Upload your compliance policies',
                  desc: 'Drop in your policy documents (PDF, Word, text). The AI reads your rulebook and immediately knows what to flag.',
                },
                {
                  n: 3,
                  title: 'Connect your Gmail mailbox',
                  desc: 'Grant read-only Gmail access. Quarantio starts scanning within seconds. Violations are quarantined automatically.',
                },
                {
                  n: 4,
                  title: 'Review and act',
                  desc: 'Your team reviews flagged emails in the quarantine dashboard. Release clean emails, reject genuine violations, export audit logs.',
                },
              ].map((s) => (
                <div key={s.n} className="hiw-step">
                  <div className="hiw-num">{s.n}</div>
                  <div>
                    <h5>{s.title}</h5>
                    <p>{s.desc}</p>
                  </div>
                </div>
              ))}
            </div>
            <div className="hiw-visual">
              <div className="hiw-email">
                <div className="hiw-email-top">
                  <span className="hiw-email-subject">Urgent payment request from CEO</span>
                  <span className="hiw-email-from">unknown@gmail.com</span>
                </div>
                <p className="hiw-email-body">Hi, I need you to process a wire transfer of $45,000 urgently. Don't mention this to anyone else...</p>
              </div>
              <div className="hiw-arrow">↓ AI scans against your policies</div>
              <div className="hiw-verdict">
                <div>
                  <div className="hiw-verdict-label">Compliance Verdict</div>
                  <div style={{ fontSize: '.75rem', color: '#6b8c7a', marginTop: '2px' }}>Social engineering · Impersonation · Urgency tactics</div>
                </div>
                <div className="hiw-verdict-badge">HIGH RISK</div>
              </div>
              <div style={{ textAlign: 'center', fontSize: '.78rem', color: 'var(--g600)', fontWeight: 600 }}>
                ✓ Quarantined before it reached the inbox
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Pricing ── */}
      <section className="lp-section" id="pricing" style={{ background: '#fff' }}>
        <div className="lp-inner">
          <div style={{ textAlign: 'center' }}>
            <p className="section-tag">Pricing</p>
            <h2 className="section-h2">Simple, transparent pricing</h2>
            <p className="section-sub" style={{ margin: '0 auto' }}>
              Start with a 14-day free trial on Starter — card required, cancel before day 14 and you won't be charged.
            </p>
          </div>
          <div className="pricing-grid">
            {[
              {
                name: 'Starter',
                price: '29',
                tagline: 'For small teams getting started',
                popular: false,
                features: ['1,000 email scans / month', 'Up to 5 team members', 'Gmail integration', '90-day audit log retention', 'Quarantine & review', 'Email notifications'],
                cta: 'Start 14-day free trial',
                solid: false,
                note: '14 days free · Cancel anytime',
              },
              {
                name: 'Pro',
                price: '99',
                tagline: 'For growing compliance teams',
                popular: true,
                features: ['10,000 email scans / month', 'Up to 25 team members', 'Gmail integration', '1-year audit log retention', 'Everything in Starter', 'Priority support'],
                cta: 'Get started',
                solid: true,
                note: null,
              },
              {
                name: 'Business',
                price: '299',
                tagline: 'For large organizations',
                popular: false,
                features: ['Unlimited email scans', 'Unlimited team members', 'Gmail integration', '3-year audit log retention', 'Everything in Pro', 'SLA & dedicated support'],
                cta: 'Get started',
                solid: false,
                note: null,
              },
            ].map((p) => (
              <div key={p.name} className={`plan-card${p.popular ? ' popular' : ''}`}>
                {p.popular && <div className="popular-badge">Most Popular</div>}
                <div className="plan-name">{p.name}</div>
                <div className="plan-price">
                  <sup>$</sup>{p.price}<sub>/mo</sub>
                </div>
                <div className="plan-tagline">{p.tagline}</div>
                <hr className="plan-divider" />
                <ul className="plan-features">
                  {p.features.map((f) => <li key={f}>{f}</li>)}
                </ul>
                <a href={G} className={`plan-cta ${p.solid ? 'plan-cta-solid' : 'plan-cta-outline'}`}>
                  {p.cta}
                </a>
                {p.note && <p className="plan-note">{p.note}</p>}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── FAQ ── */}
      <section className="lp-section" id="faq" style={{ background: 'var(--g50)', borderTop: '1px solid var(--g100)' }}>
        <div className="lp-inner">
          <div style={{ textAlign: 'center' }}>
            <p className="section-tag">FAQ</p>
            <h2 className="section-h2">Common questions</h2>
          </div>
          <div className="faq-list">
            {[
              {
                q: 'Does Quarantio require MX record changes?',
                a: 'No. Quarantio connects to Gmail via OAuth and reads emails using the Gmail API. Your email routing is completely unchanged — no MX records, no forwarding rules, no IT involvement required.',
              },
              {
                q: 'What happens to emails that are flagged?',
                a: 'Flagged emails are moved to your organization\'s quarantine dashboard. They stay there until an owner reviews them and either releases the email back to the recipient or permanently rejects it. Nothing is deleted without an explicit action.',
              },
              {
                q: 'Does Quarantio read all my emails?',
                a: 'Yes — to perform compliance scanning, Quarantio reads incoming emails via the Gmail API (read-only access). Emails that pass the scan (CLEAN verdict) are analyzed and immediately discarded. Only emails flagged as non-compliant are stored in your quarantine.',
              },
              {
                q: 'How do I define my compliance policies?',
                a: 'Upload your policy documents (PDF, Word, plain text) in the Policies section. The AI reads your rulebook and uses it to evaluate every incoming email. You can upload multiple documents and update them anytime.',
              },
              {
                q: 'Can I cancel during the free trial?',
                a: 'Yes. The 14-day trial on the Starter plan requires a card upfront, but you won\'t be charged until day 14. Cancel from the Settings page at any time before that and you\'ll never be billed. No questions asked.',
              },
              {
                q: 'Is my data secure?',
                a: 'OAuth tokens are encrypted at rest with AES-256. All data in transit uses TLS 1.2+. Payment cards are handled exclusively by Stripe — we never see card numbers. You can export or permanently delete all your data at any time from the Settings page.',
              },
            ].map((item, i) => (
              <div key={i} className="faq-item">
                <div
                  className={`faq-q${openFaq === i ? ' open' : ''}`}
                  onClick={() => setOpenFaq(openFaq === i ? null : i)}
                >
                  {item.q}
                  <svg className={`faq-chevron${openFaq === i ? ' open' : ''}`} width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </div>
                {openFaq === i && <div className="faq-a">{item.a}</div>}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Final CTA ── */}
      <section className="lp-cta">
        <div style={{ maxWidth: '600px', margin: '0 auto' }}>
          <h2>Start protecting your team's inbox today</h2>
          <p>14-day free trial on Starter. No MX changes. Cancel anytime.</p>
          <a href={G} className="btn-hero-main" style={{ display: 'inline-flex' }}>
            <GoogleIcon size={18} />
            Get started with Google — it's free
          </a>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer style={{ background: '#fff', borderTop: '1px solid var(--g100)' }}>
        <div className="footer-inner">
          <div className="footer-logo">
            <img src="/logo.png" alt="Quarantio" />
            Quarantio
          </div>
          <div className="footer-links">
            <a href="#features">Features</a>
            <a href="#pricing">Pricing</a>
            <a href="/privacy">Privacy Policy</a>
            <a href="mailto:quarantio8@gmail.com">Contact</a>
          </div>
          <p className="footer-copy">© {new Date().getFullYear()} Quarantio. All rights reserved.</p>
        </div>
      </footer>
    </div>
  );
}

function GoogleIcon({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 48 48">
      <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
      <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
      <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
      <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.18 1.48-4.97 2.31-8.16 2.31-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
    </svg>
  );
}
