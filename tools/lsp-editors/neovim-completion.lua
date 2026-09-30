-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Completion unlocks advancement; it never supplies a response payload.
local M = {}
function M.new(client, buffer, method, advance)
  local self = {disposed=false, applied=false, retryStarted=false, requestId=nil,
    observation={provenance='neovim-request-completion/0', completionCount=0,
      handlerDelivered=false, handlerResponse=vim.NIL}}
  local function try_advance()
    if self.disposed or self.retryStarted or not self.applied or self.observation.completionCount ~= 1 then return end
    self.retryStarted = true
    vim.schedule(function() if not self.disposed then advance() end end)
  end
  self.autocmd = vim.api.nvim_create_autocmd('LspRequest', {callback=function(ev)
    local data = ev.data or {}; local request = data.request or {}
    if self.disposed or not self.requestId or ev.buf ~= buffer or data.client_id ~= client or
      data.request_id ~= self.requestId or request.method ~= method or request.type ~= 'complete' then return end
    self.observation.completionCount = self.observation.completionCount + 1
    self.observation.completion = {clientId=data.client_id, requestId=data.request_id,
      buffer=ev.buf, method=request.method, type=request.type}
    try_advance()
  end})
  function self:pin(requestId)
    self.requestId=requestId
    self.observation.identity={clientId=client,requestId=requestId,buffer=buffer,method=method}
  end
  function self:edit_applied() self.applied=true; try_advance() end
  function self:handler(response)
    self.observation.handlerDelivered=true; self.observation.handlerResponse=response
  end
  function self:dispose()
    if self.disposed then return end
    self.disposed=true; vim.api.nvim_del_autocmd(self.autocmd)
  end
  return self
end
return M
