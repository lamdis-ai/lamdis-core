"use client";
import PolicySimulator from "../../../components/policies/PolicySimulator";
import { SectionTitle } from "@lamdis/ui";

export default function SimulatorPage(){
  return <div className="grid gap-6">
    <SectionTitle>Policy Simulator</SectionTitle>
    <PolicySimulator />
  </div>;
}
