// Schema for skv.cue files.

#Spec: {
	tools?: #Tools
	skills: [...#Skill]
	...
}

#Tools: {
	exclude?: [...string]
	...
}

#Skill: #Remote | #Local

#Remote: {
	name: string
	repo: string
	path?: string
	ref?:  string
	local?: ""
	...
}

#Local: {
	name:  string
	local: string
	repo?: ""
	path?: ""
	ref?:  ""
	...
}

skv: #Spec
