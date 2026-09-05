import Link from "next/link";
import { Braces } from "lucide-react";

export function Brand({ href = "/login" }: { href?: string }) {
  return <Link href={href} className="brand" aria-label="HireTech home"><span className="brand-mark"><Braces aria-hidden="true" /></span><span>Hire<span>Tech</span></span></Link>;
}
