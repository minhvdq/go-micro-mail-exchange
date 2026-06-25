import React from 'react';

const EFFECTIVE_DATE = 'June 24, 2026';
const CONTACT_EMAIL = 'quarantio8@gmail.com';

export function Terms() {
  return (
    <div style={{ fontFamily: '-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif', color: '#1a2e23', margin: 0 }}>
      <style>{`
        :root { --g50:#f0faf5; --g100:#e0f2ea; --g200:#b8e0ca; --g500:#3d9970; --g600:#2f7a58; --g700:#1e5c3f; }
        * { box-sizing:border-box; }
        .pp-nav { background:#fff; border-bottom:1px solid var(--g100); padding:0 24px; height:60px; display:flex; align-items:center; justify-content:space-between; position:sticky; top:0; z-index:100; }
        .pp-logo { display:flex; align-items:center; gap:10px; font-weight:700; font-size:1.1rem; color:var(--g700); text-decoration:none; }
        .pp-logo img { width:28px; height:28px; object-fit:contain; }
        .pp-back { font-size:.875rem; color:var(--g600); text-decoration:none; font-weight:500; }
        .pp-back:hover { color:var(--g700); }
        .pp-hero { background:linear-gradient(135deg,var(--g700) 0%,var(--g500) 100%); padding:48px 24px; text-align:center; }
        .pp-hero h1 { font-size:clamp(1.6rem,4vw,2.4rem); font-weight:800; color:#fff; margin:0 0 8px; letter-spacing:-.02em; }
        .pp-hero p { font-size:.9rem; color:rgba(255,255,255,.75); margin:0; }
        .pp-body { max-width:760px; margin:0 auto; padding:48px 24px 80px; }
        .pp-toc { background:var(--g50); border:1px solid var(--g100); border-radius:12px; padding:20px 24px; margin-bottom:40px; }
        .pp-toc h3 { font-size:.8rem; font-weight:700; text-transform:uppercase; letter-spacing:.08em; color:var(--g500); margin:0 0 12px; }
        .pp-toc ol { margin:0; padding-left:18px; }
        .pp-toc li { margin-bottom:6px; }
        .pp-toc a { font-size:.875rem; color:var(--g600); text-decoration:none; }
        .pp-toc a:hover { text-decoration:underline; }
        .pp-section { margin-bottom:40px; }
        .pp-section h2 { font-size:1.15rem; font-weight:700; color:var(--g700); border-bottom:1px solid var(--g100); padding-bottom:8px; margin-bottom:16px; }
        .pp-section p, .pp-section li { font-size:.9375rem; color:#3d5249; line-height:1.75; }
        .pp-section ul, .pp-section ol { padding-left:20px; margin:8px 0 16px; }
        .pp-section li { margin-bottom:6px; }
        .pp-section strong { color:#1a2e23; }
        .pp-highlight { background:var(--g50); border-left:3px solid var(--g500); border-radius:0 8px 8px 0; padding:12px 16px; margin:16px 0; font-size:.9rem; color:var(--g700); }
        .pp-footer { border-top:1px solid var(--g100); margin-top:48px; padding-top:24px; font-size:.825rem; color:#6b8c7a; }
      `}</style>

      {/* Nav */}
      <nav className="pp-nav">
        <a href="/" className="pp-logo">
          <img src="/logo.png" alt="Quarantio" />
          Quarantio
        </a>
        <a href="/" className="pp-back">← Back to home</a>
      </nav>

      {/* Hero */}
      <div className="pp-hero">
        <h1>Terms of Service</h1>
        <p>Effective {EFFECTIVE_DATE}</p>
      </div>

      <div className="pp-body">
        {/* TOC */}
        <div className="pp-toc">
          <h3>Contents</h3>
          <ol>
            <li><a href="#acceptance">Acceptance of Terms</a></li>
            <li><a href="#service">Description of Service</a></li>
            <li><a href="#accounts">Accounts and Eligibility</a></li>
            <li><a href="#acceptable-use">Acceptable Use</a></li>
            <li><a href="#billing">Billing and Payments</a></li>
            <li><a href="#data">Your Data</a></li>
            <li><a href="#gmail">Gmail Integration</a></li>
            <li><a href="#ip">Intellectual Property</a></li>
            <li><a href="#disclaimer">Disclaimers and Limitations</a></li>
            <li><a href="#termination">Termination</a></li>
            <li><a href="#changes">Changes to Terms</a></li>
            <li><a href="#contact">Contact</a></li>
          </ol>
        </div>

        <div className="pp-section" id="acceptance">
          <h2>1. Acceptance of Terms</h2>
          <p>By creating an account or using Quarantio ("Service"), you agree to these Terms of Service ("Terms"). If you are using the Service on behalf of an organization, you represent that you have authority to bind that organization to these Terms.</p>
          <p>If you do not agree to these Terms, do not use the Service.</p>
        </div>

        <div className="pp-section" id="service">
          <h2>2. Description of Service</h2>
          <p>Quarantio is an AI-powered email compliance platform that connects to your Gmail account via OAuth, scans inbound emails against your organization's compliance policies, and flags or quarantines messages that may violate those policies.</p>
          <p>The Service includes:</p>
          <ul>
            <li>AI-driven email scanning and classification</li>
            <li>Policy document upload and enforcement</li>
            <li>Quarantine management and audit logging</li>
            <li>Team management and role-based access</li>
            <li>Billing and subscription management</li>
          </ul>
        </div>

        <div className="pp-section" id="accounts">
          <h2>3. Accounts and Eligibility</h2>
          <p>You must be at least 18 years old and capable of forming a legally binding contract to use the Service. You are responsible for maintaining the security of your account credentials and for all activity that occurs under your account.</p>
          <p>You must provide accurate and complete information when creating your account. One organization ("tenant") may be associated with multiple user accounts under a single subscription.</p>
        </div>

        <div className="pp-section" id="acceptable-use">
          <h2>4. Acceptable Use</h2>
          <p>You agree not to:</p>
          <ul>
            <li>Use the Service for any unlawful purpose or in violation of any regulations</li>
            <li>Attempt to reverse engineer, decompile, or extract source code from the Service</li>
            <li>Interfere with or disrupt the integrity or performance of the Service</li>
            <li>Use the Service to process email content you are not authorized to access</li>
            <li>Resell or sublicense access to the Service without our written consent</li>
            <li>Upload malicious content, scripts, or policy documents designed to circumvent the Service</li>
          </ul>
        </div>

        <div className="pp-section" id="billing">
          <h2>5. Billing and Payments</h2>
          <p>Quarantio offers paid subscription plans billed monthly. Payments are processed by Stripe. By subscribing, you authorize Quarantio to charge your payment method on a recurring basis.</p>
          <ul>
            <li><strong>Free trial:</strong> Starter plan includes a 14-day free trial. Your card is charged after the trial ends unless you cancel.</li>
            <li><strong>Upgrades:</strong> Plan upgrades take effect immediately and are prorated.</li>
            <li><strong>Cancellations:</strong> You may cancel at any time. Access continues until the end of the current billing period.</li>
            <li><strong>Refunds:</strong> We do not provide refunds for partial months or unused scans.</li>
          </ul>
          <p>We reserve the right to change pricing with 30 days' notice to your registered email address.</p>
        </div>

        <div className="pp-section" id="data">
          <h2>6. Your Data</h2>
          <p>You retain ownership of all data you submit to the Service, including email content and policy documents. By using the Service, you grant Quarantio a limited license to process that data solely to provide the Service.</p>
          <p>We do not sell your data to third parties. Email content classified as CLEAN is not stored. Quarantined email content is stored encrypted and accessible only to authorized members of your organization.</p>
          <p>You may export or delete your data at any time through the Settings panel or by contacting us at <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>.</p>
        </div>

        <div className="pp-section" id="gmail">
          <h2>7. Gmail Integration</h2>
          <p>The Service connects to Gmail via Google OAuth 2.0. By connecting your Gmail account, you authorize Quarantio to:</p>
          <ul>
            <li>Read inbound email messages for compliance scanning</li>
            <li>Modify message labels (e.g., remove from Inbox) for quarantined messages</li>
            <li>Receive push notifications via Google Cloud Pub/Sub when new emails arrive</li>
          </ul>
          <p>Quarantio's use of Gmail data complies with the <a href="https://developers.google.com/terms/api-services-user-data-policy" target="_blank" rel="noreferrer">Google API Services User Data Policy</a>, including the Limited Use requirements.</p>
          <p>You may revoke Gmail access at any time from the Service dashboard or directly from your Google Account settings.</p>
        </div>

        <div className="pp-section" id="ip">
          <h2>8. Intellectual Property</h2>
          <p>The Service, including all software, algorithms, and designs, is owned by Quarantio and protected by intellectual property laws. These Terms do not grant you any rights to our trademarks, logos, or proprietary technology.</p>
          <p>Policy documents you upload remain your intellectual property. You grant us a limited license to process them solely for compliance scanning purposes.</p>
        </div>

        <div className="pp-section" id="disclaimer">
          <h2>9. Disclaimers and Limitations of Liability</h2>
          <div className="pp-highlight">
            The Service is provided "as is" without warranties of any kind. Quarantio does not guarantee that the Service will catch every policy violation or prevent every compliance incident.
          </div>
          <p>To the maximum extent permitted by law, Quarantio shall not be liable for any indirect, incidental, special, or consequential damages arising from your use of the Service, including but not limited to lost profits, data loss, or regulatory fines.</p>
          <p>Our total liability to you for any claim arising from use of the Service shall not exceed the amount you paid us in the three months preceding the claim.</p>
        </div>

        <div className="pp-section" id="termination">
          <h2>10. Termination</h2>
          <p>You may terminate your account at any time by canceling your subscription and deleting your account from Settings.</p>
          <p>We may suspend or terminate your access if you violate these Terms, fail to pay fees, or engage in conduct that we determine is harmful to other users or the Service. We will provide notice where reasonably practicable.</p>
          <p>Upon termination, your right to use the Service ceases immediately. We will delete your data within 30 days of account closure unless required by law to retain it longer.</p>
        </div>

        <div className="pp-section" id="changes">
          <h2>11. Changes to Terms</h2>
          <p>We may update these Terms from time to time. We will notify you of material changes via email or a prominent notice in the Service. Continued use after changes take effect constitutes acceptance of the updated Terms.</p>
        </div>

        <div className="pp-section" id="contact">
          <h2>12. Contact</h2>
          <p>Questions about these Terms? Contact us at <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>.</p>
        </div>

        <div className="pp-footer">
          <p>© {new Date().getFullYear()} Quarantio. These Terms of Service were last updated on {EFFECTIVE_DATE}.</p>
        </div>
      </div>
    </div>
  );
}
