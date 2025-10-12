**Context:**
You have access to a comprehensive Product Requirements Document (PRD) for EBJoy at `docs/PRD-ebjoy-reboot-2025-06-30.md` and the RFC template guide at@agent_guide_and_templates.md . Your task is to create a detailed, implementation-ready RFC document based on the RFC roadmap outlined in Section 8 of the PRD.

**Instructions:**

1. **Review Required Documentation:**
   - Thoroughly read the PRD (`docs/PRD-ebjoy-reboot-2025-06-30.md`), focusing on:
     - Section 3 (Key Features) - understand functional requirements
     - Section 5 (Technical Stack) - adhere to technology choices
     - Section 7 (Architecture Overview) - follow architectural patterns
     - Section 8 (RFC Roadmap) - identify your assigned RFC and dependencies
     - Section 15 (Technical Decisions) - understand the rationale
   - Study the RFC template in `agent_guide_and_templates.md`

2. **Research and Investigation Phase:**
   Before writing, conduct thorough research on:
   - Official documentation for all technologies involved
   - Best practices for the implementation area
   - Required libraries and dependencies
   - Similar implementations and patterns
   - Potential challenges and solutions

3. **RFC Creation Requirements:**
   - Follow the exact template from `agent_guide_and_templates.md`
   - Include specific file paths per PRD Section 7.2
   - Define atomic, testable checklist items with comprehensive test cases
   - Document all research findings and resources
   - Include code snippets and configuration examples
   - Specify exact dependency versions

4. **Quality Standards:**
   - Each checklist item must be independently verifiable
   - Test cases must cover positive and negative scenarios
   - Technical design must be implementable by any developer
   - Include all referenced documentation links
   - Maintain consistency with previous RFCs

5. **Sequential Process:**
   - Create RFCs one at a time per the roadmap order
   - Wait for review before proceeding to the next
   - Each RFC builds upon previous ones

**RFC Assignment:**
Please create **<RFC-NUMBER+TITLE>** as specified in the PRD Section 8.

**Before writing the RFC:**

1. **Review Previous RFCs:**
   - Read all previously created/completed RFCs in `docs/rfcs/` excluding the ones in archive folder
   - Note any architectural decisions or patterns established
   - Identify dependencies and interfaces your RFC must respect
   - Check for any deviations from the original PRD that affect your RFC
   - Understand what foundation has been laid for your implementation

2. **Research Specific to This RFC:**
   - Study the specific technologies and patterns required
   - Research the problem domain and best practices
   - Identify potential integration points with existing code
   - Investigate any external services or APIs needed
   - Consider security, performance, and scalability implications

3. **Plan the Implementation:**
   - Map out the file structure changes needed
   - Design the API contracts or interfaces
   - Consider the testing strategy
   - Identify potential risks or blockers

**Deliverable:**
Create the RFC document at `docs/rfcs/RFC-[NUMBER]-[kebab-case-title].md` with:
- Status: Draft
- Branch name: `rfc/[NUMBER]-[short-description]`
- Comprehensive test cases for each checklist item
- All research URLs and documentation references
- Detailed implementation notes for future RFCs

**Important Notes:**
- Include all research URLs and documentation references in the RFC
- Provide exact commands and file contents where applicable
- Ensure test cases verify both successful implementation and that nothing is broken
- Consider how this RFC sets up the foundation for subsequent RFCs
- Document all assumptions and decisions
- Include exact commands and configurations
- Ensure alignment with previous RFCs
- Update any discovered discrepancies between PRD and current state

Please proceed with creating the assigned RFC following these guidelines.
Please proceed with creating the assigned RFC following these guidelines. NOTE: We did a backend rewrite from pocketbase to echo see RFC-020 (inital design for backend rewrite). RFC-021 to RFC-029  are the design docs we used for backendend rewrite see them also to make sure were desigining and planning RFC-010 changes based on how the BE works now.