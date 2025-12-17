import { ref, onMounted, onUnmounted } from 'vue'
import { useRuntimeConfig } from '#app'

export const useSse = () => {
    const isConnected = ref(false)
    const events = ref([] as any[])
    let eventSource: EventSource | null = null

    const connect = () => {
        if (eventSource) {
            return
        }

        const config = useRuntimeConfig()
        const apiBaseUrl = config.public.apiBaseUrl
        const url = `${apiBaseUrl}/events`

        try {
            eventSource = new EventSource(url)

            eventSource.onopen = () => {
                console.log('SSE connected')
                isConnected.value = true
            }

            eventSource.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data)
                    events.value.push(data)
                    console.log('SSE event received:', data)
                    
                    // 触发全局SSE事件
                    const customEvent = new CustomEvent('sse:data-updated', { detail: data })
                    window.dispatchEvent(customEvent)
                } catch (error) {
                    console.error('Error parsing SSE event:', error)
                }
            }

            eventSource.onerror = (error) => {
                console.error('SSE error:', error)
                isConnected.value = false
            }
        } catch (error) {
            console.error('Failed to connect to SSE:', error)
        }
    }

    const disconnect = () => {
        if (eventSource) {
            eventSource.close()
            eventSource = null
            isConnected.value = false
            console.log('SSE disconnected')
        }
    }

    onMounted(() => {
        connect()
    })

    onUnmounted(() => {
        disconnect()
    })

    return {
        isConnected,
        events,
        connect,
        disconnect
    }
}