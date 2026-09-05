import Link from "next/link";
import { ArrowRight, CheckCircle2, Clock3, LockKeyhole, MonitorCheck, ShieldCheck } from "lucide-react";
import { Brand } from "@/components/layout/brand";
import { Button } from "@/components/ui/button";

function invitationHref(token?: string): string {
  return token ? `/candidate/invitation?token=${encodeURIComponent(token)}` : "/candidate/invitation";
}

export function CandidateLandingView({ token }: { token?: string }) {
  return <div className="candidate-landing">
    <header className="candidate-landing-header">
      <div className="candidate-landing-container candidate-landing-header-inner">
        <Brand href="/candidate" />
        <div className="candidate-landing-header-note"><LockKeyhole aria-hidden="true" /> Secure candidate access</div>
      </div>
    </header>

    <main>
      <section className="candidate-landing-hero candidate-landing-container">
        <div className="candidate-landing-hero-copy">
          <p className="candidate-landing-eyebrow">HireTech technical interview</p>
          <h1>Show us how you think.</h1>
          <p className="candidate-landing-lead">A focused, structured interview experience designed to give you the space to explain your decisions, solve problems, and demonstrate your strengths.</p>
          <div className="candidate-landing-actions"><Button asChild size="lg"><Link href={invitationHref(token)}>{token ? "Review invitation" : "Enter candidate area"} <ArrowRight /></Link></Button><span><Clock3 aria-hidden="true" /> Usually 30–60 minutes</span></div>
          <div className="candidate-landing-trust"><span><ShieldCheck aria-hidden="true" /> Private by design</span><span><MonitorCheck aria-hidden="true" /> Browser-based</span><span><CheckCircle2 aria-hidden="true" /> Clear next steps</span></div>
        </div>
        <div className="candidate-landing-journey" aria-label="Interview journey">
          <div className="candidate-landing-journey-heading"><span>Your interview journey</span><span>3 steps</span></div>
          <div className="candidate-landing-step active"><span>01</span><div><strong>Review your invitation</strong><p>Confirm the role, timing, and interview format.</p></div><CheckCircle2 aria-hidden="true" /></div>
          <div className="candidate-landing-step"><span>02</span><div><strong>Prepare your setup</strong><p>Check your browser, connection, and microphone.</p></div><MonitorCheck aria-hidden="true" /></div>
          <div className="candidate-landing-step"><span>03</span><div><strong>Complete the interview</strong><p>Answer each question in your own words.</p></div><ArrowRight aria-hidden="true" /></div>
          <div className="candidate-landing-journey-footer"><LockKeyhole aria-hidden="true" /><span>Your answers are sent through a protected interview session.</span></div>
        </div>
      </section>

      <section className="candidate-landing-section candidate-landing-container">
        <div className="candidate-landing-section-heading"><p className="candidate-landing-eyebrow">What to expect</p><h2>A clear process from start to finish.</h2><p>There is no dashboard to learn and no unnecessary setup. Follow the steps, focus on the conversation, and let your technical thinking come through.</p></div>
        <div className="candidate-landing-expectations"><article><span>01</span><h3>Read the context</h3><p>See the position, competency areas, and time expectations before you begin.</p></article><article><span>02</span><h3>Think out loud</h3><p>Use the answer space to explain your approach, trade-offs, and reasoning.</p></article><article><span>03</span><h3>Finish with confidence</h3><p>Save your responses securely and receive a clear submission confirmation.</p></article></div>
      </section>

      <section className="candidate-landing-proof"><div className="candidate-landing-container candidate-landing-proof-inner"><div><ShieldCheck aria-hidden="true" /><div><strong>Designed for a fairer interview</strong><p>The interview captures evidence of your technical thinking. Hiring decisions remain with the human hiring team.</p></div></div><Button asChild variant="outline"><Link href={invitationHref(token)}>Continue <ArrowRight /></Link></Button></div></section>
    </main>

    <footer className="candidate-landing-footer"><div className="candidate-landing-container"><span>© HireTech</span><span>Candidate interview experience</span></div></footer>
  </div>;
}
