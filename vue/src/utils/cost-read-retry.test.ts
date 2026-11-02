import {afterEach,describe,expect,it,vi} from 'vitest'
import {retryCostRead} from './cost-read-retry'

afterEach(()=>vi.useRealTimers())
describe('bounded cost reads',()=>{
  it('recovers a transient 502 without retrying forever',async()=>{
    vi.useFakeTimers()
    const read=vi.fn().mockRejectedValueOnce({status:502}).mockResolvedValue({data:[1]})
    const result=retryCostRead(read)
    await vi.runAllTimersAsync()
    expect(await result).toEqual({data:[1]});expect(read).toHaveBeenCalledTimes(2)
  })
  it('stops after three reads and does not retry business or permission errors',async()=>{
    vi.useFakeTimers()
    const read=vi.fn().mockRejectedValue({status:503})
    const outcome=retryCostRead(read).catch(error=>error)
    await vi.runAllTimersAsync()
    expect(await outcome).toEqual({status:503});expect(read).toHaveBeenCalledTimes(3)
    for(const status of [400,401,403,409,500]){
      const fail=vi.fn().mockRejectedValue({status})
      await expect(retryCostRead(fail)).rejects.toEqual({status});expect(fail).toHaveBeenCalledTimes(1)
    }
  })
  it('cancels a stale page read during the retry delay',async()=>{
    vi.useFakeTimers()
    const controller=new AbortController(),read=vi.fn().mockRejectedValue({status:502})
    const result=retryCostRead(read,controller.signal).catch(error=>error)
    await Promise.resolve();controller.abort();await vi.runAllTimersAsync()
    expect(await result).toMatchObject({name:'AbortError'});expect(read).toHaveBeenCalledTimes(1)
  })
})
