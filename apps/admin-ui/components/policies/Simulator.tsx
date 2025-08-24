"use client";
import { useState } from "react";
import { Card, Field, Input, Textarea, Button, Switch } from "@lamdis/ui";
import { dryRun, testExecute } from "./api";

const EXAMPLE_ALLOW = `package policy

# Example of a complex ALLOW with conditions and reasons
# - Requires machine scope orders:write
# - Blocks if order total > 500 unless user has role "manager"
# - Requires MFA if last_auth_age > 900 seconds

default decide = {"status":"BLOCKED","reasons":["no_rule_matched"]}

has_scope(scope) {
  some s
  input.scopes[s] == scope
}

is_manager {
  input.user.roles[_] == "manager"
}

allow_due_to_limits {
  to_number(input.facts.order.total) <= 500
}

allow_due_to_role {
  to_number(input.facts.order.total) > 500
  is_manager
}

needs_step_up {
  to_number(input.auth.last_auth_age) > 900
}

decide = res {
  has_scope("orders:write")
  allow_due_to_limits or allow_due_to_role
  needs_step_up
  res := {
    "status": "ALLOW_WITH_CONDITIONS",
    "reasons": ["limits_ok_or_manager", "mfa_required"],
    "conditions": {"acr": "urn:mfa", "auth_time_secs": 900}
  }
}

decide = res {
  has_scope("orders:write")
  allow_due_to_limits or allow_due_to_role
  not needs_step_up
  res := {"status": "ALLOW", "reasons": ["limits_ok_or_manager"]}
}
`;

const EXAMPLE_BLOCK = `package policy

# Example of a complex BLOCK with alternatives and needs
# - Blocks cancel if order already shipped
# - Provides alternative actions (contact_support) and needs extra inputs

default decide = {"status":"BLOCKED","reasons":["no_rule_matched"]}

already_shipped {
  input.facts.order.status == "shipped"
}

missing_reason {
  not input.inputs.reason
}

decide = res {
  already_shipped
  res := {
    "status": "BLOCKED",
    "reasons": ["order_already_shipped"],
    "alternatives": [
      {"action": "contact_support", "title": "Contact support to intercept shipment"}
    ]
  }
}

decide = res {
  missing_reason
  res := {
    "status": "NEEDS_INPUT",
    "needs": [
      {"field": "reason", "title": "Why cancel?", "type": "string", "required": true}
    ]
  }
}
`;

export default function Simulator(){
  const [actionKey, setActionKey] = useState("orders.cancel");
  const [inputs, setInputs] = useState("{\n  \"order_id\": \"123\",\n  \"reason\": \"requested_by_customer\"\n}");
  const [policy, setPolicy] = useState(EXAMPLE_ALLOW);
  const [out, setOut] = useState<any>(null);
  const [busy, setBusy] = useState(false);
  const [trace, setTrace] = useState(false);
  const [execOut, setExecOut] = useState<any>(null);

  async function run(){
    setBusy(true);
    let inp:any = {};
    try { inp = JSON.parse(inputs) } catch { setOut({ error: "inputs must be valid JSON"}); setBusy(false); return; }
  const res = await dryRun(policy, actionKey, inp, trace);
    setOut(res);
    setBusy(false);
  }
  async function runExecute(){
    setBusy(true);
    let inp:any = {};
    try { inp = JSON.parse(inputs) } catch { setOut({ error: "inputs must be valid JSON"}); setBusy(false); return; }
    try {
      const res = await testExecute(actionKey, inp, true);
      setExecOut(res);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <div className="grid gap-3">
        <div className="text-lg font-semibold">Simulator</div>
        <div className="flex gap-2 text-xs">
          <button className="btn-ghost" onClick={()=>setPolicy(EXAMPLE_ALLOW)}>Load complex ALLOW</button>
          <button className="btn-ghost" onClick={()=>setPolicy(EXAMPLE_BLOCK)}>Load complex BLOCK/NEEDS</button>
        </div>
        <div className="grid md:grid-cols-3 gap-3">
          <Field label="Action key">
            <Input value={actionKey} onChange={(e)=>setActionKey(e.target.value)} placeholder="orders.cancel" />
          </Field>
          <div className="md:col-span-2">
            <Field label="Inputs JSON">
              <Textarea rows={6} value={inputs} onChange={(e)=>setInputs(e.target.value)} />
            </Field>
          </div>
        </div>
        <Field label="Policy (Rego)">
          <Textarea rows={10} value={policy} onChange={(e)=>setPolicy(e.target.value)} />
        </Field>
        <div className="flex items-center gap-3">
          <Switch checked={trace} onChange={setTrace} label={<span className="text-sm">Trace evaluation</span>} />
          <Button onClick={run} disabled={busy}>{busy?"Running…":"Run dry-run"}</Button>
          <Button onClick={runExecute} disabled={busy}>Run execute (trace)</Button>
        </div>
        <div className="grid md:grid-cols-3 gap-3">
          <div>
            <div className="text-sm text-gray-500 mb-1">Decision</div>
            <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">
              {JSON.stringify(out?.decision || out, null, 2)}
            </pre>
          </div>
          <div>
            <div className="text-sm text-gray-500 mb-1">Facts preview</div>
            <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">
              {JSON.stringify(out?.facts_preview, null, 2)}
            </pre>
          </div>
          <div>
            <div className="text-sm text-gray-500 mb-1">Execute result</div>
            <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">
              {JSON.stringify(execOut, null, 2)}
            </pre>
          </div>
          <div>
            <div className="text-sm text-gray-500 mb-1">Trace</div>
            <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">
              {JSON.stringify(out?.trace, null, 2)}
            </pre>
          </div>
        </div>
      </div>
    </Card>
  );
}
