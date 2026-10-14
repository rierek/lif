vim.o.termguicolors=true; vim.o.number=true; vim.o.ruler=true; vim.o.laststatus=2; vim.o.showmode=true
vim.cmd('syntax on'); vim.o.background='dark'
local h=function(g,fg,extra) vim.api.nvim_set_hl(0,g,vim.tbl_extend('force',{fg=fg},extra or {})) end
h('Normal','#cdd6f4'); h('LineNr','#6c7086'); h('Comment','#7f849c',{italic=true})
h('yamlBlockMappingKey','#89b4fa'); h('yamlPlainScalar','#a6e3a1'); h('yamlKeyValueDelimiter','#9399b2'); h('yamlBlockCollectionItemStart','#9399b2'); h('yamlDocumentStart','#7f849c'); h('yamlDocumentEnd','#7f849c')
h('markdownH2','#cba6f7',{bold=true}); h('markdownH1','#cba6f7',{bold=true}); h('markdownHeadingDelimiter','#cba6f7'); h('markdownListMarker','#9399b2'); h('markdownCode','#fab387')
h('StatusLine','#cdd6f4',{bg='#45475a'}); h('StatusLineNC','#9399b2',{bg='#313244'}); h('ModeMsg','#a6e3a1',{bold=true})
h('Delimiter','#9399b2'); h('Special','#9399b2'); h('Title','#cba6f7',{bold=true}); h('Identifier','#89b4fa'); h('String','#a6e3a1'); h('Constant','#fab387'); h('Number','#fab387')
h('Folded','#7f849c'); h('NonText','#6c7086'); h('Cursor','#1e1e2e',{bg='#cdd6f4'}); h('MsgArea','#cdd6f4')
vim.api.nvim_create_autocmd('Syntax',{pattern='markdown',callback=function()
 vim.cmd([[
 syn region lifFront start=/\%^---$/ end=/^---$/ keepend contains=lifKey,lifCom,lifDash,lifDate,lifVal
 syn match lifCom /^#.*$/ contained
 syn match lifKey /^\s*[a-z_]\+\ze:/ contained nextgroup=lifColon
 syn match lifColon /:/ contained nextgroup=lifVal,lifDate skipwhite
 syn match lifVal /\(^\s*\w\+:\s\|^\s*-\s\)\@<=.*$/ contained
 syn match lifDate /\(:\s\)\@<=\d\{4}-\d\d-\d\dT\S*$/ contained
 syn match lifDash /^\s*-\s/ contained nextgroup=lifVal
 hi lifCom guifg=#7f849c gui=italic
 hi lifKey guifg=#89b4fa
 hi lifColon guifg=#9399b2
 hi lifVal guifg=#a6e3a1
 hi lifDate guifg=#fab387
 hi lifDash guifg=#9399b2
 ]])
end})
vim.api.nvim_create_autocmd('VimEnter',{callback=function() vim.treesitter.stop(0); vim.bo.syntax='markdown' end})
