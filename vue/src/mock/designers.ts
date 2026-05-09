export interface Designer {
  id: string
  name: string
  role: 'designer' | 'lead'
}

export const mockDesigners: Designer[] = [
  {
    id: 'd1',
    name: '设计师张',
    role: 'designer',
  },
  {
    id: 'd2',
    name: '设计师李',
    role: 'designer',
  },
]

