-- SPDX-License-Identifier: AGPL-3.0-or-later
-- A real builtin client probe; it does not qualify semantic methods.
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
    if vim.env.CORVINT_EDITOR_CONTEXT == '1' then
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
  on_error = function(code, err) vim.schedule(function() report.error = {code = code, message = tostring(err)}; write() end) end,
  on_exit = function(code, signal)
    vim.schedule(function()
      report.serverExit = {code = code, signal = signal}
      report.status = report.initializeResult and code == 0 and 'LIFECYCLE_OBSERVED' or 'FAILED'
      write()
      vim.cmd('qa!')
    end)
  end,
})
if not id then report.error = 'client-start-failed'; write(); vim.cmd('cquit') end
vim.defer_fn(function()
  report.status = 'TIMEOUT'; write()
  local client = vim.lsp.get_client_by_id(id)
  if client then client:stop(true) end
  vim.cmd('cquit')
end, tonumber(vim.env.CORVINT_EDITOR_TIMEOUT_MS))
