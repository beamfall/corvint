-- SPDX-License-Identifier: AGPL-3.0-or-later
local source=debug.getinfo(1,'S').source:sub(2)
source=vim.fn.fnamemodify(source,':p')
vim.fn.chdir('/tmp') -- Exercise trusted sibling loading outside the source working directory.
local module=dofile(vim.fn.fnamemodify(source,':p:h')..'/neovim-completion.lua')
local owned={}
local ok,err=pcall(function()
  local buffer=vim.api.nvim_get_current_buf()
  local function event(client,id,buf,method,kind)
    vim.api.nvim_exec_autocmds('LspRequest',{buffer=buf,data={client_id=client,request_id=id,request={type=kind,method=method}}})
  end
  local function make() local calls=0;local c=module.new(71,buffer,'corvint/context',function() calls=calls+1 end);owned[#owned+1]=c;c:pin(3);return c,function() return calls end end
  local function drain() vim.wait(10,function() return false end) end
  for _,order in ipairs({'edit-first','complete-first'}) do
    local c,n=make()
    if order=='edit-first' then c:edit_applied() end
    event(71,3,buffer,'corvint/context','complete')
    if order=='complete-first' then c:edit_applied() end
    drain();assert(n()==1 and c.observation.handlerDelivered==false and c.observation.handlerResponse==vim.NIL)
    event(71,3,buffer,'corvint/context','complete');drain();assert(n()==1 and c.observation.completionCount==2);c:dispose();c:dispose()
  end
  local c,n=make();c:edit_applied()
  for _,tuple in ipairs({{72,3,buffer,'corvint/context','complete'},{71,4,buffer,'corvint/context','complete'},{71,3,buffer,'other','complete'},{71,3,buffer,'corvint/context','pending'},{71,3,buffer,'corvint/context','cancel'}}) do event(unpack(tuple)) end
  local other=vim.api.nvim_create_buf(false,true);event(71,3,other,'corvint/context','complete');drain();assert(n()==0 and c.observation.completionCount==0);c:dispose();vim.api.nvim_buf_delete(other,{force=true})
  c,n=make();c:edit_applied();event(71,3,buffer,'corvint/context','complete');c:dispose();c:dispose();drain();assert(n()==0)
  for _,payload in ipairs({{result={state='READY'}},{error={code=-32801,message='Context stale',data={reason='CONTENT_CHANGED'}}}}) do
    c,n=make();c:edit_applied();event(71,3,buffer,'corvint/context','complete');c:handler(payload);drain();assert(n()==1 and c.observation.handlerDelivered and c.observation.handlerResponse==payload);c:dispose()
  end
  c,n=make();c:dispose();event(71,3,buffer,'corvint/context','complete');drain();assert(n()==0 and c.observation.completionCount==0)
  -- Installed RPC queues completion and error dispatch independently. Invalidate at receipt,
  -- before any additionally scheduled report operation.
  c,n=make();c:edit_applied();local error_received=false
  vim.schedule(function() event(71,3,buffer,'corvint/context','complete') end)
  vim.schedule(function() error_received=true;c:dispose();vim.schedule(function() end) end)
  drain();assert(error_received and n()==0)
  -- Sticky workflow closure also protects setup when no controller exists at error receipt.
  for _,boundary in ipairs({'error','exit'}) do
    local closed=false;local created=0;local requests=0
    vim.schedule(function() closed=true end)
    vim.schedule(function() if closed then return end;created=created+1;requests=requests+1 end)
    drain();assert(closed and created==0 and requests==0,boundary)
  end
  -- Fast-event exit logically closes first; safe API cleanup may be queued behind advance.
  local exit_closed=false;local exit_requests=0
  local exit_controller=module.new(71,buffer,'corvint/context',function() if exit_closed then return end;exit_requests=exit_requests+1 end)
  owned[#owned+1]=exit_controller;exit_controller:pin(3);exit_controller:edit_applied()
  vim.schedule(function() event(71,3,buffer,'corvint/context','complete') end)
  vim.schedule(function() exit_closed=true;vim.schedule(function() exit_controller:dispose() end) end)
  drain();assert(exit_closed and exit_requests==0 and exit_controller.disposed)
  local old_count=exit_controller.observation.completionCount
  event(71,3,buffer,'corvint/context','complete');assert(exit_controller.observation.completionCount==old_count)
  print('Actual autocmd: exact identity, once-only, queued disposal, handler preservation, missing/duplicate and cleanup controls PASS')
end)
for _,c in ipairs(owned) do c:dispose() end
if not ok then print(err);vim.cmd('cquit 1') else vim.cmd('qa!') end
