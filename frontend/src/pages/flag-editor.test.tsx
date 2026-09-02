import { describe, expect, it } from "vitest"
import { conditionToLeaf, conditionToRule, isEditableCondition, leafToCondition, ruleToCondition, type RuleRow } from "./flag-editor"

describe("leafToCondition / conditionToLeaf round-trip", () => {
  it("round-trips a plain comparison", () => {
    const leaf = { attribute: "plan", operator: "==" as const, value: "pro", negate: false }
    expect(conditionToLeaf(leafToCondition(leaf))).toEqual(leaf)
  })

  it("round-trips a negated comparison via the ! wrapper", () => {
    const leaf = { attribute: "plan", operator: "==" as const, value: "free", negate: true }
    const condition = leafToCondition(leaf)
    expect(condition).toEqual({ "!": { "==": [{ var: "plan" }, "free"] } })
    expect(conditionToLeaf(condition)).toEqual(leaf)
  })

  it("unwraps a single-element array under !", () => {
    const condition = { "!": [{ "==": [{ var: "plan" }, "free"] }] }
    expect(conditionToLeaf(condition)).toEqual({ attribute: "plan", operator: "==", value: "free", negate: true })
  })

  it("round-trips a semver comparison", () => {
    const leaf = { attribute: "appVersion", operator: "semver>" as const, value: "1.2.0", negate: false }
    expect(conditionToLeaf(leafToCondition(leaf))).toEqual(leaf)
  })

  it("round-trips a regex match", () => {
    const leaf = { attribute: "email", operator: "matches" as const, value: "^.+@acme\\.com$", negate: false }
    expect(conditionToLeaf(leafToCondition(leaf))).toEqual(leaf)
  })

  it("splits and rejoins 'in' values as a comma-separated list", () => {
    const leaf = { attribute: "country", operator: "in" as const, value: "US,BR", negate: false }
    const condition = leafToCondition(leaf)
    expect(condition).toEqual({ in: [{ var: "country" }, ["US", "BR"]] })
    expect(conditionToLeaf(condition)).toEqual(leaf)
  })

  it("round-trips 'not in' with comma-separated values", () => {
    const leaf = { attribute: "country", operator: "not in" as const, value: "US,BR", negate: false }
    const condition = leafToCondition(leaf)
    expect(condition).toEqual({ "not in": [{ var: "country" }, ["US", "BR"]] })
    expect(conditionToLeaf(condition)).toEqual(leaf)
  })

  it("round-trips 'not contains'", () => {
    const leaf = { attribute: "email", operator: "not contains" as const, value: "@acme.com", negate: false }
    const condition = leafToCondition(leaf)
    expect(condition).toEqual({ "not contains": [{ var: "email" }, "@acme.com"] })
    expect(conditionToLeaf(condition)).toEqual(leaf)
  })

})

describe("ruleToCondition / conditionToRule round-trip", () => {
  it("round-trips a single-condition rule without a combinator wrapper", () => {
    const rule: RuleRow = {
      priority: 1,
      description: "",
      combinator: "and",
      conditions: [{ attribute: "plan", operator: "==", value: "pro", negate: false }],
      variantKey: "on",
      rollout: [],
    }
    const condition = ruleToCondition(rule)
    expect(condition).toEqual({ "==": [{ var: "plan" }, "pro"] })
    expect(conditionToRule(condition)).toEqual({ combinator: "and", conditions: rule.conditions })
  })

  it("round-trips a multi-condition and/or rule, negation included", () => {
    const rule: RuleRow = {
      priority: 1,
      description: "",
      combinator: "or",
      conditions: [
        { attribute: "plan", operator: "==", value: "pro", negate: false },
        { attribute: "country", operator: "==", value: "US", negate: true },
      ],
      variantKey: "on",
      rollout: [],
    }
    const condition = ruleToCondition(rule)
    expect(conditionToRule(condition)).toEqual({
      combinator: "or",
      conditions: [
        rule.conditions[0],
        { ...rule.conditions[1], connector: "or" },
      ],
    })
  })

  it("supports one nested AND/OR group", () => {
    const rule: RuleRow = {
      priority: 1,
      description: "",
      combinator: "and",
      conditions: [
        { attribute: "plan", operator: "==", value: "pro", negate: false },
        {
          kind: "group",
          combinator: "or",
          conditions: [
            { attribute: "country", operator: "==", value: "BR", negate: false },
            { attribute: "country", operator: "==", value: "US", negate: false },
          ],
        },
      ],
      variantKey: "on",
      rollout: [],
    }

    const condition = ruleToCondition(rule)
    expect(condition).toEqual({
      and: [
        { "==": [{ var: "plan" }, "pro"] },
        {
          or: [
            { "==": [{ var: "country" }, "BR"] },
            { "==": [{ var: "country" }, "US"] },
          ],
        },
      ],
    })
    expect(conditionToRule(condition)).toEqual({
      combinator: "and",
      conditions: [
        rule.conditions[0],
        { ...rule.conditions[1], connector: "and" },
      ],
    })
  })

  it("keeps a connector on a newly added condition independent from the preceding group", () => {
    const rule: RuleRow = {
      priority: 1,
      description: "",
      combinator: "and",
      conditions: [
        { attribute: "plan", operator: "==", value: "pro", negate: false },
        {
          kind: "group",
          combinator: "or",
          conditions: [
            { attribute: "country", operator: "==", value: "BR", negate: false },
            { attribute: "country", operator: "==", value: "US", negate: false },
          ],
          connector: "and",
        },
        { attribute: "role", operator: "==", value: "admin", negate: false, connector: "or" },
      ],
      variantKey: "on",
      rollout: [],
    }

    const condition = ruleToCondition(rule)
    expect(condition).toEqual({
      or: [
        {
          and: [
            { "==": [{ var: "plan" }, "pro"] },
            {
              or: [
                { "==": [{ var: "country" }, "BR"] },
                { "==": [{ var: "country" }, "US"] },
              ],
            },
          ],
        },
        { "==": [{ var: "role" }, "admin"] },
      ],
    })
    expect(conditionToRule(condition)).toEqual({ combinator: "or", conditions: rule.conditions })
  })
})

describe("isEditableCondition", () => {
	it("accepts one nested group containing editable leaves", () => {
		expect(isEditableCondition({
			and: [
				{ "==": [{ var: "plan" }, "pro"] },
				{ or: [{ "==": [{ var: "country" }, "BR"] }, { "==": [{ var: "country" }, "US"] }] },
			],
		})).toBe(true)
	})

	it("rejects values that cannot round-trip through the editor", () => {
		expect(isEditableCondition({ "==": [{ var: "plan" }, { tier: "pro" }] })).toBe(false)
		expect(isEditableCondition({ "==": [{ var: "plan" }, { var: "expectedPlan" }] })).toBe(false)
		expect(isEditableCondition({ in: [{ var: "country" }, ["US,CA"]] })).toBe(false)
	})

	it("rejects repeated negation wrappers", () => {
		expect(isEditableCondition({ "!": { "!": { "==": [{ var: "plan" }, "pro"] } } })).toBe(false)
	})

  it("rejects nested groups so the editor preserves their original JSONLogic", () => {
    expect(isEditableCondition({
      and: [
        { "==": [{ var: "plan" }, "pro"] },
        { or: [{ "==": [{ var: "country" }, "BR"] }, { and: [{ "==": [{ var: "role" }, "admin"] }] }] },
      ],
    })).toBe(false)
  })
})
