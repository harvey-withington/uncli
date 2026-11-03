import { describe, expect, it } from 'vitest'
import { answersFor, pick, planOf, questionsOf } from './asks'

// As recorded (fixture ask-user-question).
const input = {
  questions: [
    { question: 'Which city do you prefer?', header: 'City', multiSelect: false, options: [{ label: 'Lisbon', description: 'The capital' }, { label: 'Porto' }, { label: 'Faro' }] },
    { question: 'Which meals would you like to plan?', header: 'Meals', multiSelect: true, options: [{ label: 'Breakfast' }, { label: 'Lunch' }, { label: 'Dinner' }] },
  ],
}

describe('a question from the model', () => {
  it('is read from the tool’s input', () => {
    const qs = questionsOf(input)
    expect(qs.map(q => [q.header, q.multiSelect, q.options.length])).toEqual([['City', false, 3], ['Meals', true, 3]])
    expect(questionsOf({})).toEqual([])
    expect(questionsOf(null)).toEqual([])
  })

  it('picks one, or toggles several', () => {
    const [city, meals] = questionsOf(input)
    expect(pick(city!, ['Lisbon'], 'Porto')).toEqual(['Porto'])
    expect(pick(city!, ['Porto'], 'Porto')).toEqual([])
    expect(pick(meals!, ['Breakfast'], 'Lunch')).toEqual(['Breakfast', 'Lunch'])
    expect(pick(meals!, ['Breakfast', 'Lunch'], 'Breakfast')).toEqual(['Lunch'])
  })

  it('is answered once every question has a pick or something typed under Other', () => {
    const qs = questionsOf(input)
    expect(answersFor(qs, { 'Which city do you prefer?': ['Porto'] }, {})).toBeNull()
    expect(answersFor(qs, { 'Which city do you prefer?': ['Porto'], 'Which meals would you like to plan?': ['Breakfast', 'Lunch'] }, { 'Which meals would you like to plan?': ' a picnic ' }))
      .toEqual({ 'Which city do you prefer?': 'Porto', 'Which meals would you like to plan?': 'Breakfast, Lunch, a picnic' })
    expect(answersFor(qs, {}, { 'Which city do you prefer?': 'Coimbra', 'Which meals would you like to plan?': 'none' }))
      .toEqual({ 'Which city do you prefer?': 'Coimbra', 'Which meals would you like to plan?': 'none' })
  })
})

describe('a plan from the model', () => {
  it('is its Markdown, or nothing when the CLI gave none', () => {
    expect(planOf({ plan: '# Plan\n\n1. Read' })).toBe('# Plan\n\n1. Read')
    expect(planOf({})).toBe('')
  })
})
