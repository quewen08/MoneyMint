export default defineNuxtPlugin(() => {
  // 从 localStorage 获取深色模式设置
  const getDarkModeFromStorage = () => {
    return localStorage.getItem('darkMode') === 'true'
  }

  // 应用深色模式到 DOM
  const applyDarkMode = (isDark: boolean) => {
    if (isDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }

  // 初始化深色模式
  const isDarkMode = getDarkModeFromStorage()
  applyDarkMode(isDarkMode)

  // 导出深色模式管理函数
  const toggleDarkMode = (isDark?: boolean) => {
    const newDarkMode = isDark !== undefined ? isDark : !getDarkModeFromStorage()
    localStorage.setItem('darkMode', newDarkMode.toString())
    applyDarkMode(newDarkMode)
    return newDarkMode
  }

  return {
    provide: {
      darkMode: {
        isDark: isDarkMode,
        toggle: toggleDarkMode,
        apply: applyDarkMode
      }
    }
  }
})