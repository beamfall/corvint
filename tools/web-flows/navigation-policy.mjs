export const ranks = Object.freeze({ read:1, 'write-reversible':2, 'write-irreversible':3, 'external-side-effect':4 });
export function navigationPolicy(incoming, receipt) {
  let active, failed=false;
  const origin=incoming.manifest.origin, maximum=ranks[incoming.maxEffect] || 0;
  const disposable=incoming.origins.disposable.includes(origin);
  const gap=reason=>{ failed=true;if(!receipt.gaps.includes(reason)&&receipt.gaps.length<32) receipt.gaps.push(reason); };
  const admit=effect=>!!ranks[effect] && ranks[effect]<=maximum && (ranks[effect]<3 || disposable);
  return {
    get failed(){return failed;},
    get active(){return active;},
    gap,
    activate(step){if(failed || !admit(step.effect_class)) throw new Error('effect-blocked');active=step;},
    deactivate(){active=undefined;},
    preflight(){if(failed || !active || !admit(active.effect_class)) throw new Error('effect-blocked');},
    formAllowed:maximum>=3 && disposable,
    request(url,method,form=false) {
      let effect=active?.effect_class || 'read';
      if((method!=='GET'||form) && ranks[effect]<3) effect='write-irreversible';
      let allowed=!failed && admit(effect) && (method==='GET'&&!form || !!active);
      try {const u=new URL(url);allowed=allowed && u.origin===origin && u.protocol==='http:' && !u.username && !u.password;}catch{allowed=false;}
      if(receipt.traffic.length>=8192){gap('traffic-budget');return false;}
      receipt.traffic.push({flow_id:active?.flow_id||'',step_id:active?.step_id||'',method,form_submit:form,effect,allowed});
      if(!allowed)gap(form?'form-blocked':'request-blocked');
      return allowed;
    },
  };
}
