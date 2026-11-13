// @vitest-environment jsdom
/**
 * 共享密码输入框回归（AC-36 配套）：
 * 默认必须是 password（成员管理等旧测试依赖 input[type="password"] 选择器），
 * 点击眼睛按钮在 password / text 间切换，aria-label 与 aria-pressed 同步。
 */
import { afterEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import PasswordInput from './PasswordInput.vue'

afterEach(() => {
  document.body.innerHTML = ''
})

describe('PasswordInput：显示 / 隐藏切换', () => {
  it('默认渲染为 password，按钮 aria-label 为「显示密码」', () => {
    const wrapper = mount(PasswordInput, {
      props: { modelValue: 'secret123', placeholder: '••••••••' },
      attachTo: document.body,
    })
    const input = wrapper.find('input')
    expect(input.attributes('type')).toBe('password')
    // fb-input 类必须保留（全局样式与既有测试选择器都依赖它）
    expect(input.classes()).toContain('fb-input')
    expect(input.attributes('placeholder')).toBe('••••••••')

    const button = wrapper.find('button')
    expect(button.attributes('type')).toBe('button')
    expect(button.attributes('aria-label')).toBe('显示密码')
    expect(button.attributes('aria-pressed')).toBe('false')

    wrapper.unmount()
  })

  it('点击后变 text 且 aria 同步，再点变回 password', async () => {
    const wrapper = mount(PasswordInput, {
      props: { modelValue: 'secret123' },
      attachTo: document.body,
    })

    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(wrapper.find('input').attributes('type')).toBe('text')
    expect(wrapper.find('button').attributes('aria-label')).toBe('隐藏密码')
    expect(wrapper.find('button').attributes('aria-pressed')).toBe('true')
    // 切换后焦点仍在输入框上
    expect(document.activeElement).toBe(wrapper.find('input').element)

    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(wrapper.find('input').attributes('type')).toBe('password')
    expect(wrapper.find('button').attributes('aria-label')).toBe('显示密码')
    expect(wrapper.find('button').attributes('aria-pressed')).toBe('false')

    wrapper.unmount()
  })

  it('输入同步 v-model，disabled 时按钮一并禁用', async () => {
    const wrapper = mount(PasswordInput, {
      props: { modelValue: '', disabled: true },
    })
    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
    expect(wrapper.find('input').attributes('disabled')).toBeDefined()

    await wrapper.setProps({ disabled: false })
    await wrapper.find('input').setValue('abcd1234')
    expect(wrapper.emitted('update:modelValue')).toEqual([['abcd1234']])

    wrapper.unmount()
  })
})
