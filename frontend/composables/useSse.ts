import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useRuntimeConfig, useNuxtApp } from '#app'

// 创建全局单例变量
let sseInstance: ReturnType<typeof createSseInstance> | null = null

const createSseInstance = () => {
    const isConnected = ref(false)
    const isConnecting = ref(false) // 添加连接中标志，防止并发连接
    const events = ref([] as any[])
    let source: Response | null = null
    let reader: ReadableStreamDefaultReader | undefined = undefined
    let decoder: TextDecoder | null = null
    let reconnectTimeout: number | null = null
    const { $api } = useNuxtApp()
    const { user, authToken } = $api

    const connect = () => {
        // 如果未登录，不执行连接
        if (!user.value) {
            return
        }

        // 如果已经连接或正在连接，则不执行
        if (isConnected.value || isConnecting.value) {
            return
        }

        // 如果存在旧的连接，先关闭它
        if (source) {
            source.body?.cancel()
            source = null
        }

        // 如果存在旧的读取器，先关闭它
        if (reader) {
            reader.cancel()
            reader = undefined
        }

        // 如果存在旧的解码器，先关闭它
        if (decoder) {
            decoder = null
        }

        // 设置连接中标志
        isConnecting.value = true


        const config = useRuntimeConfig()
        const apiBaseUrl = config.public.apiBaseUrl
        const url = `${apiBaseUrl}/events` // 不需要重复添加/api前缀，因为apiBaseUrl已经包含了

        try {
            fetch(url, {
                method: 'GET',
                headers: {
                    'Authorization': authToken.value ? `Bearer ${authToken.value}` : '',
                    'Accept': 'text/event-stream'
                },
                credentials: 'include'
            }).then(async (response) => {
                if (!response.ok) {
                    isConnecting.value = false // 连接失败，重置连接中标志
                    throw new Error(`SSE connection failed: ${response.status} ${response.statusText}`)
                }

                source = response
                decoder = new TextDecoder()
                reader = response.body?.getReader()

                if (!reader) {
                    throw new Error('Failed to get reader from response body')
                }

                isConnected.value = true
                isConnecting.value = false // 连接成功，重置连接中标志
                console.log('SSE connected')

                // 开始读取SSE事件
                readEvents()
            }).catch((error) => {
                console.error('Failed to connect to SSE:', error)
                isConnected.value = false
                // 尝试重连
                scheduleReconnect()
            })
        } catch (error) {
            console.error('Failed to connect to SSE:', error)
            isConnected.value = false
            isConnecting.value = false // 连接失败，重置连接中标志
            // 尝试重连
            scheduleReconnect()
        }
    }

    const readEvents = async () => {
        if (!reader) return

        try {
            let buffer = ''
            while (true) {
                const { done, value } = await reader.read()
                if (done) {
                    console.log('SSE connection closed by server')
                    isConnected.value = false
                    // 尝试重连
                    scheduleReconnect()
                    break
                }

                buffer += decoder?.decode(value, { stream: true }) || ''

                // 分割事件
                const eventLines = buffer.split('\n\n')
                buffer = eventLines.pop() || ''

                for (const eventLine of eventLines) {
                    if (!eventLine.trim()) continue

                    // 解析SSE事件格式
                    const dataLines = eventLine.split('\n').filter(line => line.startsWith('data:'))
                    if (dataLines.length > 0) {
                        const data = dataLines.map(line => line.substring(5).trim()).join('\n')
                        try {
                            const parsedData = JSON.parse(data)
                            events.value.push(parsedData)
                            console.log('SSE event received:', parsedData)

                            // 触发全局SSE事件
                            const customEvent = new CustomEvent('sse:data-updated', { detail: parsedData })
                            window.dispatchEvent(customEvent)
                        } catch (parseError) {
                            console.error('Error parsing SSE event:', parseError)
                        }
                    }
                }
            }
        } catch (error) {
            console.error('Error reading SSE events:', error)
            isConnected.value = false
            // 尝试重连
            scheduleReconnect()
        }
    }

    const scheduleReconnect = () => {
        if (reconnectTimeout) {
            clearTimeout(reconnectTimeout)
        }
        // 5秒后尝试重连
        reconnectTimeout = window.setTimeout(() => {
            console.log('Attempting to reconnect to SSE...')
            connect()
        }, 5000)
    }

    const disconnect = () => {
        if (reconnectTimeout) {
            clearTimeout(reconnectTimeout)
            reconnectTimeout = null
        }

        if (reader) {
            reader.cancel()
            reader = undefined
        }

        source = null
        decoder = null
        isConnected.value = false
        console.log('SSE disconnected')
    }

    // 监听用户登录状态变化，登录成功后重连SSE
    watch(user, (newUser, oldUser) => {
        // 如果用户从无到有（登录），重新连接SSE
        if (newUser && !oldUser) {
            disconnect() // 确保先断开旧连接
            connect() // 重新建立连接
        }
        // 如果用户从有到无（登出），断开SSE连接
        else if (!newUser && oldUser) {
            disconnect()
        }
    })

    onMounted(() => {
        // 初始连接SSE
        connect()
    })

    // 组件卸载时不要断开连接，因为单例需要在整个应用生命周期中保持

    return {
        isConnected,
        events,
        connect,
        disconnect
    }
}

export const useSse = () => {
    // 如果还没有创建实例，则创建一个新实例
    if (!sseInstance) {
        sseInstance = createSseInstance()
    }

    // 返回单例实例
    return sseInstance
}