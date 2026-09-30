-- SPDX-License-Identifier: AGPL-3.0-or-later
-- A real builtin client probe; it does not qualify semantic methods.
local source = debug.getinfo(1, 'S').source:sub(2)
local completion_module = dofile(vim.fn.fnamemodify(source, ':p:h') .. '/neovim-completion.lua')
local completion
local closed = false
local function dispose() closed = true; if completion then completion:dispose() end end
local report = {client = 'neovim', version = vim.version(), status = 'FAILED'}
local function write()
  vim.fn.writefile({vim.json.encode(report)}, vim.env.CORVINT_EDITOR_RESULT)
end
vim.cmd.edit(vim.env.CORVINT_EDITOR_DOCUMENT)
vim.bo.filetype = 'go'
local buffer = vim.api.nvim_get_current_buf()
local id = vim.lsp.start({
  name = 'corvint-experimental-qualification',
  cmd = vim.json.decode(vim.env.CORVINT_EDITOR_SERVER_ARGV),
  root_dir = vim.env.CORVINT_EDITOR_ROOT,
  flags = {debounce_text_changes = 0},
  on_init = function(client, result)
    report.initializeResult = result
    report.encoding = client.offset_encoding
    report.root = client.config.root_dir
    report.status = 'INITIALIZED'
    write()
    if vim.env.CORVINT_EDITOR_FRESHNESS == '1' then
      local params = {textDocument = {uri = vim.uri_from_bufnr(buffer)}, task = vim.env.CORVINT_EDITOR_CONTEXT_TASK, limit = 20}
      local function observed(err, result)
        if err then return {error = {code = err.code, message = err.message, data = err.data}} end
        return {result = result}
      end
      vim.defer_fn(function()
        if closed then return end
        vim.api.nvim_buf_set_lines(buffer, 0, -1, false, {'package p', '/*😀*/ var x int', 'var y = x'})
        vim.defer_fn(function()
          if closed then return end
          client:request('corvint/context', params, function(aerr, baseline, acontext)
            vim.schedule(function()
              if closed then return end
              local pending = true
              report.freshness = {baseline = observed(aerr, baseline), versions = {acontext.version}}
              completion = completion_module.new(client.id, buffer, 'corvint/context', function()
                if closed then return end
                client:request('corvint/context', params, function(berr, retry, bcontext)
                  vim.schedule(function()
                    if closed then return end
                    report.freshness.retry = observed(berr, retry)
                    report.freshness.versions[2] = bcontext.version
                    report.freshness.bufferModified = vim.bo[buffer].modified
                    dispose(); write(); client:stop()
                  end)
                end, buffer)
              end)
              report.freshness.rapid = completion.observation
              local sent, request_id = client:request('corvint/context', params, function(rerr, rapid)
                pending = false
                completion:handler(observed(rerr, rapid))
              end, buffer)
              if sent then completion:pin(request_id) end
              report.freshness.pendingBeforeEdit = pending
              if not sent then dispose(); report.error = 'rapid-request-not-sent'; write(); client:stop(); return end
              vim.api.nvim_buf_set_lines(buffer, 0, -1, false, {'package p', '/*😀*/ var newest int', 'var y = newest'})
              completion:edit_applied()
            end)
          end, buffer)
        end, 200)
      end, 200)
    elseif vim.env.CORVINT_EDITOR_CONTEXT == '1' then
      vim.defer_fn(function()
        vim.api.nvim_buf_set_lines(buffer, 0, -1, false, {'package p', '/*😀*/ var x int', 'var y = x'})
        vim.defer_fn(function()
          client:request('corvint/context', {textDocument = {uri = vim.uri_from_bufnr(buffer)}, task = vim.env.CORVINT_EDITOR_CONTEXT_TASK, limit = 20},
            function(err, receipt, context)
              vim.schedule(function()
                report.context = {error = err, result = receipt, version = context.version, bufferModified = vim.bo[buffer].modified}
                write(); client:stop()
              end)
            end, buffer)
        end, 200)
      end, 200)
    elseif vim.env.CORVINT_EDITOR_SEMANTIC == '1' then
      vim.defer_fn(function()
        vim.api.nvim_buf_set_lines(buffer, 0, -1, false, {'package p', '/*😀*/ var x int', 'var y = x'})
        vim.defer_fn(function()
          local position = {line = 2, character = vim.str_utfindex('var y = ', client.offset_encoding)}
          client:request('textDocument/definition', {textDocument = {uri = vim.uri_from_bufnr(buffer)}, position = position},
            function(err, definition, context)
              vim.schedule(function()
                report.semantic = {error = err, definition = definition, requestPosition = position, bufferModified = vim.bo[buffer].modified, version = context.version}
                write(); client:stop()
              end)
            end, buffer)
        end, 200)
      end, 200)
    else
      vim.defer_fn(function() client:stop() end, 100)
    end
  end,
  on_error = function(code, err) dispose(); vim.schedule(function() report.error = {code = code, message = tostring(err)}; write() end) end,
  on_exit = function(code, signal)
    closed = true -- Invalidate immediately; API cleanup runs in the scheduled safe context.
    vim.schedule(function()
      dispose()
      report.serverExit = {code = code, signal = signal}
      report.status = report.initializeResult and code == 0 and 'LIFECYCLE_OBSERVED' or 'FAILED'
      write()
      vim.cmd('qa!')
    end)
  end,
})
if not id then report.error = 'client-start-failed'; write(); vim.cmd('cquit') end
vim.defer_fn(function()
  dispose()
  report.status = 'TIMEOUT'; write()
  local client = vim.lsp.get_client_by_id(id)
  if client then client:stop(true) end
  vim.cmd('cquit')
end, tonumber(vim.env.CORVINT_EDITOR_TIMEOUT_MS))
