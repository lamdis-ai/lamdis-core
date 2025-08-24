"use client";
import PolicyWorkbench from "../../../components/policies/PolicyWorkbench";
import { SectionTitle } from "@lamdis/ui";

export default function WorkbenchPage(){
  return <div className="grid gap-6">
    <SectionTitle>Policy Workbench</SectionTitle>
    <PolicyWorkbench />
  </div>;
}
