local function run_ifacegen(path)
    local binary_path = vim.fn.expand('./../bin/run.exe')

    local command = binary_path .. " " .. path

    vim.fn.jobstart(command, { detach = true })
end

vim.api.nvim_create_autocmd("BufWritePost", {
    pattern = "*.go",
    callback = function()
        local file_path = vim.fn.expand('%:p:h')
        run_ifacegen(file_path)
    end
})
